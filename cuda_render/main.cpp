#include "cuda_color.hpp"
#include "cuda_overlay.hpp"
#include "ff_audio.hpp"

#include <algorithm>
#include <chrono>
#include <cmath>
#include <cstdio>
#include <cstdlib>
#include <cstring>
#include <sstream>
#include <string>
#include <vector>
#include <windows.h>
#include <shellapi.h>

extern "C" {
#include <libavcodec/avcodec.h>
#include <libavcodec/bsf.h>
#include <libavfilter/avfilter.h>
#include <libavfilter/buffersink.h>
#include <libavfilter/buffersrc.h>
#include <libavformat/avformat.h>
#include <libavutil/avutil.h>
#include <libavutil/hwcontext.h>
#include <libavutil/opt.h>
}

struct TrimSeg { double start = 0.0, end = 1e12; };

struct Options {
    std::string input;
    std::string output;
    std::string color = "none";
    std::string codec = "h264";
    std::string preset = "p4";
    int cq = 28;
    int maxrateKbps = 0;
    int padRight = 0;
    double speed = 1.0;
    std::string atempo;
    std::string vfilter;
    std::vector<TrimSeg> trims;
    std::vector<CudaOverlay> overlays;
};

static bool parseOverlaySpec(const std::string& spec, CudaOverlay& ov) {
    std::vector<std::string> parts;
    std::stringstream ss(spec);
    std::string item;
    while (std::getline(ss, item, '|')) parts.push_back(item);
    if (parts.size() < 7) return false;
    ov.path = parts[0];
    ov.targetW = std::atoi(parts[1].c_str());
    ov.xRatio = std::atof(parts[2].c_str());
    ov.yRatio = std::atof(parts[3].c_str());
    ov.opacity = std::atof(parts[4].c_str());
    ov.startSec = std::atof(parts[5].c_str());
    ov.endSec = std::atof(parts[6].c_str());
    return !ov.path.empty();
}

static void parseTrimSpec(const std::string& spec, std::vector<TrimSeg>& trims) {
    std::stringstream ss(spec);
    std::string seg;
    while (std::getline(ss, seg, ',')) {
        auto dash = seg.find('-');
        if (dash == std::string::npos) continue;
        TrimSeg ts;
        ts.start = std::atof(seg.substr(0, dash).c_str());
        ts.end = std::atof(seg.substr(dash + 1).c_str());
        if (ts.end > ts.start) trims.push_back(ts);
    }
}

static std::vector<std::string> getUtf8Args() {
    std::vector<std::string> out;
    int wargc = 0;
    LPWSTR* wargv = CommandLineToArgvW(GetCommandLineW(), &wargc);
    if (!wargv) return out;
    for (int i = 0; i < wargc; ++i) {
        int len = WideCharToMultiByte(CP_UTF8, 0, wargv[i], -1, nullptr, 0, nullptr, nullptr);
        std::string s(len > 0 ? len - 1 : 0, '\0');
        if (len > 1) WideCharToMultiByte(CP_UTF8, 0, wargv[i], -1, s.data(), len, nullptr, nullptr);
        out.push_back(std::move(s));
    }
    LocalFree(wargv);
    return out;
}

static bool parseArgs(const std::vector<std::string>& args, Options& opt) {
    for (size_t i = 1; i < args.size(); ++i) {
        const std::string& k = args[i];
        auto next = [&]() -> std::string { return (i + 1 < args.size()) ? args[++i] : ""; };
        if (k == "--input") opt.input = next();
        else if (k == "--output") opt.output = next();
        else if (k == "--color") opt.color = next();
        else if (k == "--codec") opt.codec = next();
        else if (k == "--preset") opt.preset = next();
        else if (k == "--cq") opt.cq = std::atoi(next().c_str());
        else if (k == "--maxrate") opt.maxrateKbps = std::atoi(next().c_str());
        else if (k == "--pad-right") opt.padRight = std::atoi(next().c_str());
        else if (k == "--speed") opt.speed = std::atof(next().c_str());
        else if (k == "--atempo") opt.atempo = next();
        else if (k == "--vfilter") opt.vfilter = next();
        else if (k == "--trim") parseTrimSpec(next(), opt.trims);
        else if (k == "--overlay") {
            CudaOverlay ov;
            if (parseOverlaySpec(next(), ov)) opt.overlays.push_back(ov);
        }
    }
    if (opt.speed <= 0.0) opt.speed = 1.0;
    return !opt.input.empty() && !opt.output.empty();
}

static enum AVPixelFormat getCudaFormat(AVCodecContext* ctx, const enum AVPixelFormat* pix_fmts) {
    for (const enum AVPixelFormat* p = pix_fmts; *p != AV_PIX_FMT_NONE; ++p) {
        if (*p == AV_PIX_FMT_CUDA) return *p;
    }
    return AV_PIX_FMT_NONE;
}

int main() {
    SetConsoleOutputCP(CP_UTF8);
    SetDllDirectoryA("D:\\code\\environment\\ffmpeg-shared-9.0.2\\bin");
    Options opt;
    if (!parseArgs(getUtf8Args(), opt)) {
        std::fprintf(stderr, "Usage: velocity-cuda --input <in> --output <out> [--color sepia] ...\n");
        return 2;
    }

    AVFormatContext* inFmt = nullptr;
    if (avformat_open_input(&inFmt, opt.input.c_str(), nullptr, nullptr) < 0 ||
        avformat_find_stream_info(inFmt, nullptr) < 0) {
        std::fprintf(stderr, "Failed to open input: %s\n", opt.input.c_str());
        return 1;
    }

    int vIdx = av_find_best_stream(inFmt, AVMEDIA_TYPE_VIDEO, -1, -1, nullptr, 0);
    int aIdx = av_find_best_stream(inFmt, AVMEDIA_TYPE_AUDIO, -1, -1, nullptr, 0);
    if (vIdx < 0) {
        std::fprintf(stderr, "No video stream found\n");
        return 1;
    }

    const AVCodec* dec = nullptr;
    if (inFmt->streams[vIdx]->codecpar->codec_id == AV_CODEC_ID_AV1) {
        dec = avcodec_find_decoder_by_name("av1");
    }
    if (!dec) dec = avcodec_find_decoder(inFmt->streams[vIdx]->codecpar->codec_id);
    if (!dec) {
        std::fprintf(stderr, "No decoder found for video stream\n");
        return 1;
    }
    AVCodecContext* decCtx = avcodec_alloc_context3(dec);
    avcodec_parameters_to_context(decCtx, inFmt->streams[vIdx]->codecpar);

    AVBufferRef* hwDevCtx = nullptr;
    if (av_hwdevice_ctx_create(&hwDevCtx, AV_HWDEVICE_TYPE_CUDA, nullptr, nullptr, 0) < 0) {
        std::fprintf(stderr, "Failed to create CUDA hwdevice\n");
        return 1;
    }
    decCtx->hw_device_ctx = av_buffer_ref(hwDevCtx);
    decCtx->extra_hw_frames = 8;
    decCtx->get_format = getCudaFormat;
    if (avcodec_open2(decCtx, dec, nullptr) < 0) {
        std::fprintf(stderr, "Failed to open video decoder with CUDA\n");
        return 1;
    }

    CudaColorProcessor colorProc;
    std::string colorErr;
    if (!colorProc.init(hwDevCtx, opt.color, !opt.overlays.empty(), colorErr) ||
        !colorProc.uploadOverlays(opt.overlays, colorErr)) {
        std::fprintf(stderr, "CudaColorProcessor init error: %s\n", colorErr.c_str());
        return 1;
    }

    AVFormatContext* outFmt = nullptr;
    if (avformat_alloc_output_context2(&outFmt, nullptr, "mp4", opt.output.c_str()) < 0) {
        std::fprintf(stderr, "Failed to alloc output context\n");
        return 1;
    }

    const bool isHEVC = (opt.codec == "hevc" || opt.codec == "h265");
    const AVCodec* enc = avcodec_find_encoder_by_name(isHEVC ? "hevc_nvenc" : "h264_nvenc");
    AVStream* outVStream = avformat_new_stream(outFmt, nullptr);
    AVCodecContext* encCtx = avcodec_alloc_context3(enc);

    const bool hasTrim = !opt.trims.empty();
    AVStream* inAStream = (aIdx >= 0) ? inFmt->streams[aIdx] : nullptr;
    AVStream* outAStream = nullptr;
    AudioTranscoder audioTx;
    bool useAudioTx = false;
    if (inAStream) {
        if (canStreamCopyAudio(inAStream, opt.speed, hasTrim)) {
            outAStream = avformat_new_stream(outFmt, nullptr);
            avcodec_parameters_copy(outAStream->codecpar, inAStream->codecpar);
            outAStream->codecpar->codec_tag = 0;
            outAStream->time_base = inAStream->time_base;
        } else {
            std::string aErr;
            if (!audioTx.init(inAStream, outFmt, opt.atempo, aErr)) {
                std::fprintf(stderr, "AudioTranscoder init error: %s\n", aErr.c_str());
                return 1;
            }
            useAudioTx = true;
        }
    }

    AVFilterGraph* fg = nullptr;
    AVFilterContext *bufSrc = nullptr, *bufSink = nullptr;
    AVBSFContext* bsfCtx = nullptr;
    bool bypassFilter = false;
    bool pipelineReady = false;

    auto initFilterAndEncoder = [&](AVFrame* firstCudaFrame) -> bool {
        AVRational guessFr = av_guess_frame_rate(inFmt, inFmt->streams[vIdx], nullptr);
        if (guessFr.num <= 0 || guessFr.den <= 0) guessFr = AVRational{30, 1};

        auto* fc = firstCudaFrame->hw_frames_ctx
            ? reinterpret_cast<AVHWFramesContext*>(firstCudaFrame->hw_frames_ctx->data)
            : nullptr;
        bypassFilter = opt.vfilter.empty() && fc && fc->sw_format == AV_PIX_FMT_NV12 &&
                       (firstCudaFrame->width % 2 == 0) && (firstCudaFrame->height % 2 == 0);

        if (bypassFilter) {
            encCtx->width = firstCudaFrame->width;
            encCtx->height = firstCudaFrame->height;
            encCtx->pix_fmt = AV_PIX_FMT_CUDA;
            encCtx->time_base = inFmt->streams[vIdx]->time_base;
            encCtx->framerate = guessFr;
            encCtx->sample_aspect_ratio = firstCudaFrame->sample_aspect_ratio;
            encCtx->color_range = firstCudaFrame->color_range;
            encCtx->colorspace = firstCudaFrame->colorspace;
            encCtx->hw_frames_ctx = av_buffer_ref(firstCudaFrame->hw_frames_ctx);
        } else {
            fg = avfilter_graph_alloc();
            const AVFilter* srcF = avfilter_get_by_name("buffer");
            const AVFilter* sinkF = avfilter_get_by_name("buffersink");
            bufSrc = avfilter_graph_alloc_filter(fg, srcF, "in");
            AVBufferSrcParameters* par = av_buffersrc_parameters_alloc();
            par->format = AV_PIX_FMT_CUDA;
            par->width = firstCudaFrame->width;
            par->height = firstCudaFrame->height;
            par->time_base = inFmt->streams[vIdx]->time_base;
            par->frame_rate = guessFr;
            par->sample_aspect_ratio = firstCudaFrame->sample_aspect_ratio;
            par->color_range = firstCudaFrame->color_range;
            par->color_space = firstCudaFrame->colorspace;
            par->hw_frames_ctx = av_buffer_ref(firstCudaFrame->hw_frames_ctx);
            if (av_buffersrc_parameters_set(bufSrc, par) < 0) return false;
            av_free(par);
            if (avfilter_init_str(bufSrc, nullptr) < 0) return false;

            if (avfilter_graph_create_filter(&bufSink, sinkF, "out", nullptr, nullptr, fg) < 0) return false;
            std::string fDesc = opt.vfilter.empty()
                ? "scale_cuda=w=trunc(iw/2)*2:h=trunc(ih/2)*2:format=yuv420p"
                : opt.vfilter;
            AVFilterInOut *outputs = avfilter_inout_alloc(), *inputs = avfilter_inout_alloc();
            outputs->name = av_strdup("in");  outputs->filter_ctx = bufSrc;  outputs->pad_idx = 0; outputs->next = nullptr;
            inputs->name  = av_strdup("out"); inputs->filter_ctx  = bufSink; inputs->pad_idx  = 0; inputs->next  = nullptr;
            if (avfilter_graph_parse_ptr(fg, fDesc.c_str(), &inputs, &outputs, nullptr) < 0 ||
                avfilter_graph_config(fg, nullptr) < 0) {
                std::fprintf(stderr, "Filtergraph config failed: %s\n", fDesc.c_str());
                return false;
            }
            avfilter_inout_free(&inputs);
            avfilter_inout_free(&outputs);

            encCtx->width = av_buffersink_get_w(bufSink);
            encCtx->height = av_buffersink_get_h(bufSink);
            encCtx->pix_fmt = AV_PIX_FMT_CUDA;
            encCtx->time_base = av_buffersink_get_time_base(bufSink);
            encCtx->framerate = av_buffersink_get_frame_rate(bufSink);
            if (encCtx->framerate.num <= 0 || encCtx->framerate.den <= 0) encCtx->framerate = guessFr;
            encCtx->hw_frames_ctx = av_buffer_ref(av_buffersink_get_hw_frames_ctx(bufSink));
        }
        if (isHEVC) encCtx->codec_tag = MKTAG('h', 'v', 'c', '1');
        encCtx->bit_rate = 0;
        if (opt.maxrateKbps > 0) {
            encCtx->rc_max_rate = static_cast<int64_t>(opt.maxrateKbps) * 1000;
            encCtx->rc_buffer_size = encCtx->rc_max_rate * 3;
        }
        if (outFmt->oformat->flags & AVFMT_GLOBALHEADER) encCtx->flags |= AV_CODEC_FLAG_GLOBAL_HEADER;
        AVDictionary* encOpts = nullptr;
        av_dict_set(&encOpts, "preset", opt.preset.c_str(), 0);
        av_dict_set(&encOpts, "rc", "vbr", 0);
        av_dict_set(&encOpts, "cq", std::to_string(opt.cq).c_str(), 0);
        av_dict_set(&encOpts, "spatial_aq", "1", 0);
        int openRc = avcodec_open2(encCtx, enc, &encOpts);
        av_dict_free(&encOpts);
        if (openRc < 0) return false;
        avcodec_parameters_from_context(outVStream->codecpar, encCtx);
        outVStream->time_base = encCtx->time_base;

        if (opt.padRight > 0) {
            const AVBitStreamFilter* bsf = av_bsf_get_by_name(isHEVC ? "hevc_metadata" : "h264_metadata");
            if (bsf && av_bsf_alloc(bsf, &bsfCtx) >= 0) {
                avcodec_parameters_copy(bsfCtx->par_in, outVStream->codecpar);
                av_opt_set_int(bsfCtx->priv_data, "crop_right", opt.padRight, 0);
                av_bsf_init(bsfCtx);
                avcodec_parameters_copy(outVStream->codecpar, bsfCtx->par_out);
            }
        }
        if (!(outFmt->oformat->flags & AVFMT_NOFILE) && avio_open(&outFmt->pb, opt.output.c_str(), AVIO_FLAG_WRITE) < 0) return false;
        AVDictionary* muxOpts = nullptr;
        av_dict_set(&muxOpts, "movflags", "+faststart", 0);
        int hdr = avformat_write_header(outFmt, &muxOpts);
        av_dict_free(&muxOpts);
        return hdr >= 0;
    };

    auto writeEncPkt = [&](AVPacket* epkt) {
        av_packet_rescale_ts(epkt, encCtx->time_base, outVStream->time_base);
        epkt->stream_index = outVStream->index;
        if (bsfCtx) {
            av_bsf_send_packet(bsfCtx, epkt);
            while (av_bsf_receive_packet(bsfCtx, epkt) == 0) {
                av_interleaved_write_frame(outFmt, epkt);
                av_packet_unref(epkt);
            }
        } else {
            av_interleaved_write_frame(outFmt, epkt);
            av_packet_unref(epkt);
        }
    };

    AVPacket* pkt = av_packet_alloc();
    AVPacket* encPkt = av_packet_alloc();
    AVFrame* decFrame = av_frame_alloc();
    AVFrame* filtFrame = av_frame_alloc();
    int64_t frameCount = 0;
    int64_t lastOutPts = AV_NOPTS_VALUE;
    double vTb = av_q2d(inFmt->streams[vIdx]->time_base);
    double lastOutSecs = 0.0;
    auto renderStart = std::chrono::steady_clock::now();

    struct BufferedAudio { AVPacket* pkt; double s, e, offset; };
    std::vector<BufferedAudio> earlyAudio;

    auto flushEarlyAudio = [&]() {
        for (auto& ba : earlyAudio) {
            if (outAStream) {
                av_packet_rescale_ts(ba.pkt, inAStream->time_base, outAStream->time_base);
                ba.pkt->stream_index = outAStream->index;
                av_interleaved_write_frame(outFmt, ba.pkt);
            } else if (useAudioTx) {
                audioTx.feedPacket(ba.pkt, ba.s, ba.e, ba.offset, true);
            }
            av_packet_free(&ba.pkt);
        }
        earlyAudio.clear();
    };

    auto encodeAndReport = [&](AVFrame* f) {
        if (avcodec_send_frame(encCtx, f) >= 0) {
            while (avcodec_receive_packet(encCtx, encPkt) == 0) writeEncPkt(encPkt);
        }
        ++frameCount;
        if (f->pts != AV_NOPTS_VALUE) {
            double secs = f->pts * av_q2d(encCtx->time_base);
            if (secs > lastOutSecs) lastOutSecs = secs;
            if (frameCount == 1 || (frameCount % 15) == 0) {
                double elapsed = std::chrono::duration<double>(std::chrono::steady_clock::now() - renderStart).count();
                double curFps = (elapsed > 0.001) ? (static_cast<double>(frameCount) / elapsed) : 0.0;
                double curSpd = (elapsed > 0.001) ? (secs / elapsed) : 0.0;
                std::printf("fps=%.1f\nspeed=%.2fx\nout_time_us=%lld\nprogress=continue\n",
                            curFps, curSpd, static_cast<long long>(secs * 1e6));
                std::fflush(stdout);
            }
        }
    };

    auto processCudaFrame = [&](AVFrame* cf, double outT) -> bool {
        AVFrame* colored = colorProc.apply(cf, outT, opt.overlays, colorErr);
        if (!colored) return false;
        int64_t outPts = static_cast<int64_t>(std::llround(outT / vTb));
        if (lastOutPts != AV_NOPTS_VALUE && outPts <= lastOutPts) outPts = lastOutPts + 1;
        lastOutPts = outPts;
        colored->pts = outPts;

        if (!pipelineReady) {
            pipelineReady = initFilterAndEncoder(colored);
            if (!pipelineReady) { av_frame_free(&colored); return false; }
            flushEarlyAudio();
        }
        if (bypassFilter) {
            encodeAndReport(colored);
            av_frame_free(&colored);
            return true;
        }
        if (av_buffersrc_add_frame_flags(bufSrc, colored, 0) < 0) { av_frame_free(&colored); return false; }
        av_frame_free(&colored);
        while (av_buffersink_get_frame(bufSink, filtFrame) >= 0) {
            encodeAndReport(filtFrame);
            av_frame_unref(filtFrame);
        }
        return true;
    };

    std::vector<TrimSeg> segments = hasTrim ? opt.trims : std::vector<TrimSeg>{{0.0, 1e12}};
    double keptOffset = 0.0;

    for (size_t si = 0; si < segments.size(); ++si) {
        const TrimSeg& seg = segments[si];
        if (hasTrim) {
            avcodec_flush_buffers(decCtx);
            if (useAudioTx) audioTx.resetDecoderForSeek();
            int64_t seekTarget = static_cast<int64_t>(std::max(0.0, seg.start) / vTb);
            av_seek_frame(inFmt, vIdx, seekTarget, AVSEEK_FLAG_BACKWARD);
        }

        bool videoDone = false;
        while (!videoDone && av_read_frame(inFmt, pkt) >= 0) {
            if (pkt->stream_index == vIdx) {
                if (avcodec_send_packet(decCtx, pkt) >= 0) {
                    while (avcodec_receive_frame(decCtx, decFrame) == 0) {
                        double t = (decFrame->pts != AV_NOPTS_VALUE) ? decFrame->pts * vTb : 0.0;
                        if (hasTrim && t < seg.start - 0.001) {
                            av_frame_unref(decFrame);
                            continue;
                        }
                        if (hasTrim && t >= seg.end - 0.001) {
                            videoDone = true;
                            av_frame_unref(decFrame);
                            break;
                        }
                        double keptT = keptOffset + std::max(0.0, t - seg.start);
                        double outT = keptT / opt.speed;
                        if (!processCudaFrame(decFrame, outT)) return 1;
                        av_frame_unref(decFrame);
                    }
                }
            } else if (pkt->stream_index == aIdx && (outAStream || useAudioTx)) {
                if (!pipelineReady) {
                    earlyAudio.push_back({av_packet_clone(pkt), seg.start, seg.end, keptOffset});
                } else if (outAStream) {
                    av_packet_rescale_ts(pkt, inAStream->time_base, outAStream->time_base);
                    pkt->stream_index = outAStream->index;
                    av_interleaved_write_frame(outFmt, pkt);
                } else if (useAudioTx) {
                    audioTx.feedPacket(pkt, seg.start, seg.end, keptOffset, true);
                }
            }
            av_packet_unref(pkt);
        }
        if (hasTrim) {
            avcodec_send_packet(decCtx, nullptr);
            while (!videoDone && avcodec_receive_frame(decCtx, decFrame) == 0) {
                double t = (decFrame->pts != AV_NOPTS_VALUE) ? decFrame->pts * vTb : 0.0;
                if (t >= seg.start - 0.001 && t < seg.end - 0.001) {
                    double outT = (keptOffset + (t - seg.start)) / opt.speed;
                    processCudaFrame(decFrame, outT);
                }
                av_frame_unref(decFrame);
            }
            keptOffset += (seg.end - seg.start);
        }
    }

    if (!hasTrim) {
        avcodec_send_packet(decCtx, nullptr);
        while (avcodec_receive_frame(decCtx, decFrame) == 0) {
            double t = (decFrame->pts != AV_NOPTS_VALUE) ? decFrame->pts * vTb : 0.0;
            processCudaFrame(decFrame, t / opt.speed);
            av_frame_unref(decFrame);
        }
    }
    if (pipelineReady) {
        if (useAudioTx) audioTx.flush(true);
        if (!bypassFilter && bufSrc && av_buffersrc_add_frame_flags(bufSrc, nullptr, 0) >= 0) {
            while (av_buffersink_get_frame(bufSink, filtFrame) >= 0) {
                encodeAndReport(filtFrame);
                av_frame_unref(filtFrame);
            }
        }
        avcodec_send_frame(encCtx, nullptr);
        while (avcodec_receive_packet(encCtx, encPkt) == 0) writeEncPkt(encPkt);
        av_write_trailer(outFmt);
    }
    double elapsed = std::chrono::duration<double>(std::chrono::steady_clock::now() - renderStart).count();
    double curFps = (elapsed > 0.001) ? (static_cast<double>(frameCount) / elapsed) : 0.0;
    double curSpd = (elapsed > 0.001) ? (lastOutSecs / elapsed) : 0.0;
    std::printf("fps=%.1f\nspeed=%.2fx\nout_time_us=%lld\nprogress=end\n",
                curFps, curSpd, static_cast<long long>(lastOutSecs * 1e6));
    std::fflush(stdout);
    return 0;
}
