package ffmpeg

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"velocityengine/internal/model"
	"velocityengine/internal/syswin"
)

const (
	defaultLocalFFmpeg   = `D:\code\environment\ffmpeg\bin\ffmpeg.exe`
	defaultLocalFFprobe  = `D:\code\environment\ffmpeg\bin\ffprobe.exe`
	defaultWingetFFmpeg  = `C:\Users\8hung\AppData\Local\Microsoft\WinGet\Packages\Gyan.FFmpeg_Microsoft.Winget.Source_8wekyb3d8bbwe\ffmpeg-8.1.1-full_build\bin\ffmpeg.exe`
	defaultWingetFFprobe = `C:\Users\8hung\AppData\Local\Microsoft\WinGet\Packages\Gyan.FFmpeg_Microsoft.Winget.Source_8wekyb3d8bbwe\ffmpeg-8.1.1-full_build\bin\ffprobe.exe`
)

// BinaryPaths holds resolved executable paths for FFmpeg and FFprobe.
type BinaryPaths struct {
	FFmpeg  string
	FFprobe string
}

// ResolveBinaries resolves ffmpeg and ffprobe paths from env, local FFmpeg 9+, WinGet, or PATH.
func ResolveBinaries() BinaryPaths {
	ff := resolveOne([]string{os.Getenv("FFMPEG_PATH"), defaultLocalFFmpeg, defaultWingetFFmpeg}, "ffmpeg")
	fp := resolveOne([]string{os.Getenv("FFPROBE_PATH"), defaultLocalFFprobe, defaultWingetFFprobe}, "ffprobe")
	if fp == "" && ff != "" {
		sibling := filepath.Join(filepath.Dir(ff), "ffprobe.exe")
		if _, err := os.Stat(sibling); err == nil {
			fp = sibling
		}
	}
	return BinaryPaths{FFmpeg: ff, FFprobe: fp}
}

func resolveOne(candidates []string, name string) string {
	for _, candidate := range candidates {
		if candidate != "" {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	return ""
}

// DetectCapabilities inspects available FFmpeg tools, NVIDIA GPU presence, and NVENC encoders.
func DetectCapabilities(ctx context.Context, bins BinaryPaths) model.SystemCapabilities {
	caps := model.SystemCapabilities{
		FFmpegAvailable:  bins.FFmpeg != "",
		FFprobeAvailable: bins.FFprobe != "",
	}
	gpuName := detectNvidiaGPU(ctx)
	if gpuName != "" {
		caps.HasNvidiaGPU = true
		caps.GPUName = gpuName
	}
	if bins.FFmpeg != "" {
		h264, hevc := detectNVENCEncoders(ctx, bins.FFmpeg)
		caps.NvencH264 = caps.HasNvidiaGPU && h264
		caps.NvencHEVC = caps.HasNvidiaGPU && hevc
	}
	return caps
}

func detectNvidiaGPU(ctx context.Context) string {
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cctx, "nvidia-smi", "--query-gpu=name", "--format=csv,noheader")
	cmd.SysProcAttr = syswin.HiddenProcAttr()
	if out, err := cmd.Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			name := strings.TrimSpace(line)
			if name != "" {
				return name
			}
		}
	}

	// Fallback to WMI/CIM VideoController query on Windows.
	wmiCmd := exec.CommandContext(cctx, "powershell.exe", "-NoProfile", "-NonInteractive", "-Command",
		"Get-CimInstance Win32_VideoController | Select-Object -ExpandProperty Name")
	wmiCmd.SysProcAttr = syswin.HiddenProcAttr()
	if out, err := wmiCmd.Output(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			name := strings.TrimSpace(line)
			if strings.Contains(strings.ToUpper(name), "NVIDIA") {
				return name
			}
		}
	}
	return ""
}

func detectNVENCEncoders(ctx context.Context, ffmpegPath string) (h264 bool, hevc bool) {
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(cctx, ffmpegPath, "-hide_banner", "-v", "error", "-encoders")
	cmd.SysProcAttr = syswin.HiddenProcAttr()
	out, err := cmd.Output()
	if err != nil {
		return false, false
	}
	text := string(out)
	return strings.Contains(text, "h264_nvenc"), strings.Contains(text, "hevc_nvenc")
}

// MapCPUPreset maps NVENC presets (p1..p7) to libx264/libx265 presets.
func MapCPUPreset(preset string) string {
	p := strings.ToLower(strings.TrimSpace(preset))
	switch p {
	case "p1", "p2", "p3":
		return "veryfast"
	case "p4", "p5", "":
		return "medium"
	case "p6", "p7":
		return "slow"
	default:
		return p
	}
}

// IsCUDADecodeError checks if stderr indicates a CUDA/NVDEC decode or hwaccel failure.
func IsCUDADecodeError(stderr string) bool {
	low := strings.ToLower(stderr)
	patterns := []string{
		"cuda", "cuvid", "nvdec", "hwaccel", "hwupload", "hwdownload",
		"av_hwdevice_ctx_create", "nvcuda", "impossible to convert between the formats",
		"function not implemented", "vulkan", "libplacebo", "no such filter",
	}
	for _, p := range patterns {
		if strings.Contains(low, p) {
			return true
		}
	}
	return false
}

// IsHardwareNVENCError checks if stderr indicates missing/unsupported NVIDIA/CUDA/NVENC hardware.
func IsHardwareNVENCError(stderr string) bool {
	low := strings.ToLower(stderr)
	patterns := []string{
		"cuda_error", "cuda error", "no cuda", "nvenc",
		"cannot load nvcuda", "no capable devices", "hardware device",
		"openencodesession", "minimum required nvidia driver",
		"unsupported device", "device creation failed",
	}
	for _, p := range patterns {
		if strings.Contains(low, p) {
			return true
		}
	}
	return false
}
