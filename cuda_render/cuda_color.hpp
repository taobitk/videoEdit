#pragma once

#include "cuda_overlay.hpp"

#include <algorithm>
#include <cmath>
#include <cstdio>
#include <cstring>
#include <string>
#include <vector>
#include <windows.h>

extern "C" {
#include <libavutil/frame.h>
#include <libavutil/hwcontext.h>
#include <libavutil/hwcontext_cuda.h>
}

class CudaColorProcessor {
public:
    ~CudaColorProcessor() { cleanup(); }

    bool init(AVBufferRef* hwDevCtx, const std::string& filterName, bool hasOverlays, std::string& errOut) {
        mode_ = parseMode(filterName);
        if (mode_ == 0 && !hasOverlays) return true;

        AddDllDirectory(L"D:\\code\\environment\\cuda-12.8\\bin");
        SetDllDirectoryA("D:\\code\\environment\\cuda-12.8\\bin");
        hNvrtc_ = LoadLibraryA("nvrtc64_120_0.dll");
        hCuda_ = LoadLibraryA("nvcuda.dll");
        if (!hNvrtc_ || !hCuda_) {
            errOut = "Failed to load nvrtc64_120_0.dll or nvcuda.dll";
            return false;
        }
        if (!loadSymbols(errOut)) return false;

        auto* avCuda = reinterpret_cast<AVCUDADeviceContext*>(
            reinterpret_cast<AVHWDeviceContext*>(hwDevCtx->data)->hwctx);
        cuCtx_ = avCuda->cuda_ctx;
        cuStream_ = avCuda->stream;

        if (cuCtxPushCurrent_(cuCtx_) != 0) {
            errOut = "cuCtxPushCurrent failed";
            return false;
        }
        int dev = 0, major = 8, minor = 6;
        cuCtxGetDevice_(&dev);
        cuDeviceGetAttribute_(&major, 75, dev);
        cuDeviceGetAttribute_(&minor, 76, dev);
        char archOpt[32];
        std::snprintf(archOpt, sizeof(archOpt), "--gpu-architecture=compute_%d%d", major, minor);

        void* prog = nullptr;
        if (nvrtcCreateProgram_(&prog, kernelSource(), "color.cu", 0, nullptr, nullptr) != 0) {
            cuCtxPopCurrent_(nullptr);
            errOut = "nvrtcCreateProgram failed";
            return false;
        }
        const char* opts[] = {archOpt};
        int rc = nvrtcCompileProgram_(prog, 1, opts);
        if (rc != 0) {
            size_t logSz = 0;
            nvrtcGetProgramLogSize_(prog, &logSz);
            std::string log(logSz, '\0');
            nvrtcGetProgramLog_(prog, log.data());
            nvrtcDestroyProgram_(&prog);
            cuCtxPopCurrent_(nullptr);
            errOut = "NVRTC compile failed: " + log;
            return false;
        }
        size_t ptxSz = 0;
        nvrtcGetPTXSize_(prog, &ptxSz);
        std::vector<char> ptx(ptxSz);
        nvrtcGetPTX_(prog, ptx.data());
        nvrtcDestroyProgram_(&prog);

        int mrc = cuModuleLoadData_(&cuMod_, ptx.data());
        int frc = (mrc == 0) ? cuModuleGetFunction_(&cuFn_, cuMod_, "nv12_color_filter") : mrc;
        int orc = (frc == 0) ? cuModuleGetFunction_(&cuOverlayFn_, cuMod_, "nv12_overlay_blend") : frc;
        cuCtxPopCurrent_(nullptr);
        if (mrc != 0 || frc != 0 || orc != 0) {
            errOut = "cuModuleLoadData / cuModuleGetFunction failed";
            return false;
        }
        return true;
    }

    bool uploadOverlays(std::vector<CudaOverlay>& overlays, std::string& errOut) {
        if (overlays.empty()) return true;
        if (cuCtxPushCurrent_(cuCtx_) != 0) {
            errOut = "cuCtxPushCurrent failed for overlays";
            return false;
        }
        for (auto& ov : overlays) {
            std::vector<uint8_t> rgba;
            if (!loadOverlayRGBA(ov.path, ov.targetW, rgba, ov.w, ov.h, errOut)) {
                cuCtxPopCurrent_(nullptr);
                return false;
            }
            if (cuMemAlloc_(&ov.dPtr, rgba.size()) != 0 ||
                cuMemcpyHtoD_(ov.dPtr, rgba.data(), rgba.size()) != 0) {
                cuCtxPopCurrent_(nullptr);
                errOut = "Failed to upload overlay to CUDA VRAM";
                return false;
            }
            uploadedPtrs_.push_back(ov.dPtr);
        }
        cuCtxPopCurrent_(nullptr);
        return true;
    }

    AVFrame* apply(
        AVFrame* inFrame,
        double outTimeSec,
        const std::vector<CudaOverlay>& overlays,
        std::string& errOut)
    {
        if (!inFrame || !inFrame->hw_frames_ctx) return av_frame_clone(inFrame);
        bool hasActiveOv = false;
        for (const auto& ov : overlays) {
            if (ov.dPtr && outTimeSec >= ov.startSec && outTimeSec <= ov.endSec) {
                hasActiveOv = true;
                break;
            }
        }
        if (mode_ == 0 && !hasActiveOv) {
            return av_frame_clone(inFrame);
        }
        auto* fc = reinterpret_cast<AVHWFramesContext*>(inFrame->hw_frames_ctx->data);
        if (fc->sw_format != AV_PIX_FMT_NV12) {
            return av_frame_clone(inFrame);
        }

        AVFrame* outFrame = av_frame_alloc();
        if (!outFrame) return nullptr;
        if (av_hwframe_get_buffer(inFrame->hw_frames_ctx, outFrame, 0) < 0 ||
            av_frame_copy_props(outFrame, inFrame) < 0) {
            av_frame_free(&outFrame);
            errOut = "av_hwframe_get_buffer failed";
            return nullptr;
        }
        if (cuCtxPushCurrent_(cuCtx_) != 0) {
            av_frame_free(&outFrame);
            return nullptr;
        }

        unsigned long long srcY = reinterpret_cast<uintptr_t>(inFrame->data[0]);
        unsigned long long srcUV = reinterpret_cast<uintptr_t>(inFrame->data[1]);
        unsigned long long dstY = reinterpret_cast<uintptr_t>(outFrame->data[0]);
        unsigned long long dstUV = reinterpret_cast<uintptr_t>(outFrame->data[1]);
        int srcPitchY = inFrame->linesize[0], srcPitchUV = inFrame->linesize[1];
        int dstPitchY = outFrame->linesize[0], dstPitchUV = outFrame->linesize[1];
        int w = inFrame->width, h = inFrame->height;
        int mode = mode_;

        void* args[] = {&srcY, &srcUV, &srcPitchY, &srcPitchUV,
                        &dstY, &dstUV, &dstPitchY, &dstPitchUV, &w, &h, &mode};
        unsigned gx = static_cast<unsigned>(((w + 1) / 2 + 15) / 16);
        unsigned gy = static_cast<unsigned>(((h + 1) / 2 + 15) / 16);
        int lrc = cuLaunchKernel_(cuFn_, gx, gy, 1, 16, 16, 1, 0, cuStream_, args, nullptr);

        for (const auto& ov : overlays) {
            if (lrc != 0) break;
            if (!ov.dPtr || outTimeSec < ov.startSec || outTimeSec > ov.endSec) continue;
            int ox = static_cast<int>(std::llround(w * ov.xRatio - ov.w * 0.5));
            int oy = static_cast<int>(std::llround(h * ov.yRatio - ov.h * 0.5));
            int startX = std::max(0, ox) & ~1;
            int startY = std::max(0, oy) & ~1;
            int endX = std::min(w, ox + ov.w);
            int endY = std::min(h, oy + ov.h);
            if (startX >= endX || startY >= endY) continue;

            unsigned long long dOv = ov.dPtr;
            int ow = ov.w, oh = ov.h;
            float op = static_cast<float>(ov.opacity);
            void* oArgs[] = {&dstY, &dstUV, &dstPitchY, &dstPitchUV, &w, &h,
                             &dOv, &ow, &oh, &ox, &oy, &startX, &startY, &op};
            unsigned ogx = static_cast<unsigned>(((endX - startX + 1) / 2 + 15) / 16);
            unsigned ogy = static_cast<unsigned>(((endY - startY + 1) / 2 + 15) / 16);
            lrc = cuLaunchKernel_(cuOverlayFn_, ogx, ogy, 1, 16, 16, 1, 0, cuStream_, oArgs, nullptr);
        }

        cuCtxPopCurrent_(nullptr);
        if (lrc != 0) {
            av_frame_free(&outFrame);
            errOut = "cuLaunchKernel failed";
            return nullptr;
        }
        return outFrame;
    }

private:
    static int parseMode(const std::string& name) {
        if (name == "grayscale") return 1;
        if (name == "sepia") return 2;
        if (name == "warm") return 3;
        if (name == "cool") return 4;
        if (name == "vibrant") return 5;
        return 0;
    }

    static const char* kernelSource() {
        return R"(
__device__ __forceinline__ unsigned char clamp_u8(float v, float lo, float hi) {
    v = v < lo ? lo : (v > hi ? hi : v);
    return static_cast<unsigned char>(v + 0.5f);
}

__device__ __forceinline__ void apply_color(float& r, float& g, float& b, int mode) {
    if (mode == 0) return;
    float nr = r, ng = g, nb = b;
    if (mode == 1) {
        float l = 0.2126f * r + 0.7152f * g + 0.0722f * b;
        nr = ng = nb = l;
    } else if (mode == 2) {
        nr = 0.393f * r + 0.769f * g + 0.189f * b;
        ng = 0.349f * r + 0.686f * g + 0.168f * b;
        nb = 0.272f * r + 0.534f * g + 0.131f * b;
    } else if (mode == 3) {
        nr = 1.12f * r; ng = 1.02f * g; nb = 0.86f * b;
    } else if (mode == 4) {
        nr = 0.86f * r; ng = 1.00f * g; nb = 1.14f * b;
    } else if (mode == 5) {
        float l = 0.2126f * r + 0.7152f * g + 0.0722f * b;
        nr = l + 1.4f * (r - l);
        ng = l + 1.4f * (g - l);
        nb = l + 1.4f * (b - l);
    }
    r = nr < 0.0f ? 0.0f : (nr > 1.0f ? 1.0f : nr);
    g = ng < 0.0f ? 0.0f : (ng > 1.0f ? 1.0f : ng);
    b = nb < 0.0f ? 0.0f : (nb > 1.0f ? 1.0f : nb);
}

extern "C" __global__ void nv12_color_filter(
    const unsigned char* __restrict__ srcY, const unsigned char* __restrict__ srcUV,
    int srcPitchY, int srcPitchUV,
    unsigned char* __restrict__ dstY, unsigned char* __restrict__ dstUV,
    int dstPitchY, int dstPitchUV, int w, int h, int mode)
{
    int tx = blockIdx.x * blockDim.x + threadIdx.x;
    int ty = blockIdx.y * blockDim.y + threadIdx.y;
    int bx = tx * 2, by = ty * 2;
    if (bx >= w || by >= h) return;

    if (mode == 0) {
        dstUV[ty * dstPitchUV + 2 * tx]     = srcUV[ty * srcPitchUV + 2 * tx];
        dstUV[ty * dstPitchUV + 2 * tx + 1] = srcUV[ty * srcPitchUV + 2 * tx + 1];
        for (int dy = 0; dy < 2; ++dy) {
            int py = by + dy;
            if (py >= h) continue;
            for (int dx = 0; dx < 2; ++dx) {
                int px = bx + dx;
                if (px < w) dstY[py * dstPitchY + px] = srcY[py * srcPitchY + px];
            }
        }
        return;
    }

    float u = (static_cast<float>(srcUV[ty * srcPitchUV + 2 * tx]) - 128.0f) / 224.0f;
    float v = (static_cast<float>(srcUV[ty * srcPitchUV + 2 * tx + 1]) - 128.0f) / 224.0f;
    float uSum = 0.0f, vSum = 0.0f;
    int cnt = 0;

    for (int dy = 0; dy < 2; ++dy) {
        int py = by + dy;
        if (py >= h) continue;
        for (int dx = 0; dx < 2; ++dx) {
            int px = bx + dx;
            if (px >= w) continue;
            float y = (static_cast<float>(srcY[py * srcPitchY + px]) - 16.0f) / 219.0f;
            float r = y + 1.5748f * v;
            float g = y - 0.1873f * u - 0.4681f * v;
            float b = y + 1.8556f * u;
            apply_color(r, g, b, mode);
            float ny = 16.0f + 219.0f * (0.2126f * r + 0.7152f * g + 0.0722f * b);
            float nu = 128.0f + 224.0f * (-0.1146f * r - 0.3854f * g + 0.5000f * b);
            float nv = 128.0f + 224.0f * (0.5000f * r - 0.4542f * g - 0.0458f * b);
            dstY[py * dstPitchY + px] = clamp_u8(ny, 16.0f, 235.0f);
            uSum += nu; vSum += nv; cnt++;
        }
    }
    if (cnt > 0) {
        dstUV[ty * dstPitchUV + 2 * tx]     = clamp_u8(uSum / cnt, 16.0f, 240.0f);
        dstUV[ty * dstPitchUV + 2 * tx + 1] = clamp_u8(vSum / cnt, 16.0f, 240.0f);
    }
}

extern "C" __global__ void nv12_overlay_blend(
    unsigned char* __restrict__ dstY, unsigned char* __restrict__ dstUV,
    int dstPitchY, int dstPitchUV, int w, int h,
    const unsigned char* __restrict__ ovRgba, int ow, int oh,
    int ox, int oy, int startX, int startY, float opacity)
{
    int bx = startX + (blockIdx.x * blockDim.x + threadIdx.x) * 2;
    int by = startY + (blockIdx.y * blockDim.y + threadIdx.y) * 2;
    if (bx >= w || by >= h || bx >= ox + ow || by >= oy + oh) return;

    int cx = bx >> 1, cy = by >> 1;
    float u = (static_cast<float>(dstUV[cy * dstPitchUV + 2 * cx]) - 128.0f) / 224.0f;
    float v = (static_cast<float>(dstUV[cy * dstPitchUV + 2 * cx + 1]) - 128.0f) / 224.0f;
    float uSum = 0.0f, vSum = 0.0f;
    int cnt = 0;

    for (int dy = 0; dy < 2; ++dy) {
        int py = by + dy;
        if (py >= h) continue;
        for (int dx = 0; dx < 2; ++dx) {
            int px = bx + dx;
            if (px >= w) continue;
            float y = (static_cast<float>(dstY[py * dstPitchY + px]) - 16.0f) / 219.0f;
            float r = y + 1.5748f * v;
            float g = y - 0.1873f * u - 0.4681f * v;
            float b = y + 1.8556f * u;

            int lx = px - ox, ly = py - oy;
            if (lx >= 0 && lx < ow && ly >= 0 && ly < oh) {
                int idx = (ly * ow + lx) * 4;
                float a = (static_cast<float>(ovRgba[idx + 3]) / 255.0f) * opacity;
                float or_ = static_cast<float>(ovRgba[idx]) / 255.0f;
                float og_ = static_cast<float>(ovRgba[idx + 1]) / 255.0f;
                float ob_ = static_cast<float>(ovRgba[idx + 2]) / 255.0f;
                r = r * (1.0f - a) + or_ * a;
                g = g * (1.0f - a) + og_ * a;
                b = b * (1.0f - a) + ob_ * a;
            }
            float ny = 16.0f + 219.0f * (0.2126f * r + 0.7152f * g + 0.0722f * b);
            float nu = 128.0f + 224.0f * (-0.1146f * r - 0.3854f * g + 0.5000f * b);
            float nv = 128.0f + 224.0f * (0.5000f * r - 0.4542f * g - 0.0458f * b);
            dstY[py * dstPitchY + px] = clamp_u8(ny, 16.0f, 235.0f);
            uSum += nu; vSum += nv; cnt++;
        }
    }
    if (cnt > 0) {
        dstUV[cy * dstPitchUV + 2 * cx]     = clamp_u8(uSum / cnt, 16.0f, 240.0f);
        dstUV[cy * dstPitchUV + 2 * cx + 1] = clamp_u8(vSum / cnt, 16.0f, 240.0f);
    }
}
)";
    }

    bool loadSymbols(std::string& errOut) {
        nvrtcCreateProgram_ = reinterpret_cast<decltype(nvrtcCreateProgram_)>(GetProcAddress(hNvrtc_, "nvrtcCreateProgram"));
        nvrtcCompileProgram_ = reinterpret_cast<decltype(nvrtcCompileProgram_)>(GetProcAddress(hNvrtc_, "nvrtcCompileProgram"));
        nvrtcGetPTXSize_ = reinterpret_cast<decltype(nvrtcGetPTXSize_)>(GetProcAddress(hNvrtc_, "nvrtcGetPTXSize"));
        nvrtcGetPTX_ = reinterpret_cast<decltype(nvrtcGetPTX_)>(GetProcAddress(hNvrtc_, "nvrtcGetPTX"));
        nvrtcGetProgramLogSize_ = reinterpret_cast<decltype(nvrtcGetProgramLogSize_)>(GetProcAddress(hNvrtc_, "nvrtcGetProgramLogSize"));
        nvrtcGetProgramLog_ = reinterpret_cast<decltype(nvrtcGetProgramLog_)>(GetProcAddress(hNvrtc_, "nvrtcGetProgramLog"));
        nvrtcDestroyProgram_ = reinterpret_cast<decltype(nvrtcDestroyProgram_)>(GetProcAddress(hNvrtc_, "nvrtcDestroyProgram"));

        cuCtxPushCurrent_ = reinterpret_cast<decltype(cuCtxPushCurrent_)>(GetProcAddress(hCuda_, "cuCtxPushCurrent_v2"));
        cuCtxPopCurrent_ = reinterpret_cast<decltype(cuCtxPopCurrent_)>(GetProcAddress(hCuda_, "cuCtxPopCurrent_v2"));
        cuCtxGetDevice_ = reinterpret_cast<decltype(cuCtxGetDevice_)>(GetProcAddress(hCuda_, "cuCtxGetDevice"));
        cuDeviceGetAttribute_ = reinterpret_cast<decltype(cuDeviceGetAttribute_)>(GetProcAddress(hCuda_, "cuDeviceGetAttribute"));
        cuModuleLoadData_ = reinterpret_cast<decltype(cuModuleLoadData_)>(GetProcAddress(hCuda_, "cuModuleLoadData"));
        cuModuleGetFunction_ = reinterpret_cast<decltype(cuModuleGetFunction_)>(GetProcAddress(hCuda_, "cuModuleGetFunction"));
        cuModuleUnload_ = reinterpret_cast<decltype(cuModuleUnload_)>(GetProcAddress(hCuda_, "cuModuleUnload"));
        cuLaunchKernel_ = reinterpret_cast<decltype(cuLaunchKernel_)>(GetProcAddress(hCuda_, "cuLaunchKernel"));
        cuMemAlloc_ = reinterpret_cast<decltype(cuMemAlloc_)>(GetProcAddress(hCuda_, "cuMemAlloc_v2"));
        cuMemcpyHtoD_ = reinterpret_cast<decltype(cuMemcpyHtoD_)>(GetProcAddress(hCuda_, "cuMemcpyHtoD_v2"));
        cuMemFree_ = reinterpret_cast<decltype(cuMemFree_)>(GetProcAddress(hCuda_, "cuMemFree_v2"));

        if (!nvrtcCreateProgram_ || !nvrtcCompileProgram_ || !cuCtxPushCurrent_ ||
            !cuModuleLoadData_ || !cuLaunchKernel_ || !cuMemAlloc_ || !cuMemcpyHtoD_) {
            errOut = "Missing required NVRTC/CUDA driver symbols";
            return false;
        }
        return true;
    }

    void cleanup() {
        if (cuCtx_ && cuCtxPushCurrent_ && cuCtxPushCurrent_(cuCtx_) == 0) {
            if (cuMemFree_) {
                for (auto p : uploadedPtrs_) cuMemFree_(p);
            }
            uploadedPtrs_.clear();
            if (cuMod_ && cuModuleUnload_) {
                cuModuleUnload_(cuMod_);
                cuMod_ = nullptr;
            }
            cuCtxPopCurrent_(nullptr);
        }
        if (hNvrtc_) { FreeLibrary(hNvrtc_); hNvrtc_ = nullptr; }
        if (hCuda_) { FreeLibrary(hCuda_); hCuda_ = nullptr; }
    }

    int mode_ = 0;
    HMODULE hNvrtc_ = nullptr, hCuda_ = nullptr;
    CUcontext cuCtx_ = nullptr;
    CUstream cuStream_ = nullptr;
    void* cuMod_ = nullptr;
    void* cuFn_ = nullptr;
    void* cuOverlayFn_ = nullptr;
    std::vector<unsigned long long> uploadedPtrs_;

    int (*nvrtcCreateProgram_)(void**, const char*, const char*, int, const char**, const char**) = nullptr;
    int (*nvrtcCompileProgram_)(void*, int, const char**) = nullptr;
    int (*nvrtcGetPTXSize_)(void*, size_t*) = nullptr;
    int (*nvrtcGetPTX_)(void*, char*) = nullptr;
    int (*nvrtcGetProgramLogSize_)(void*, size_t*) = nullptr;
    int (*nvrtcGetProgramLog_)(void*, char*) = nullptr;
    int (*nvrtcDestroyProgram_)(void**) = nullptr;

    int (*cuCtxPushCurrent_)(CUcontext) = nullptr;
    int (*cuCtxPopCurrent_)(CUcontext*) = nullptr;
    int (*cuCtxGetDevice_)(int*) = nullptr;
    int (*cuDeviceGetAttribute_)(int*, int, int) = nullptr;
    int (*cuModuleLoadData_)(void**, const void*) = nullptr;
    int (*cuModuleGetFunction_)(void**, void*, const char*) = nullptr;
    int (*cuModuleUnload_)(void*) = nullptr;
    int (*cuLaunchKernel_)(void*, unsigned,unsigned,unsigned, unsigned,unsigned,unsigned, unsigned, CUstream, void**, void**) = nullptr;
    int (*cuMemAlloc_)(unsigned long long*, size_t) = nullptr;
    int (*cuMemcpyHtoD_)(unsigned long long, const void*, size_t) = nullptr;
    int (*cuMemFree_)(unsigned long long) = nullptr;
};
