package ffmpeg

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"velocityengine/internal/mathutil"
	"velocityengine/internal/model"
)

var cudaPathOnce sync.Once

// ResolveCudaRunnerPath locates velocity-cuda.exe and always returns an absolute path.
func ResolveCudaRunnerPath() string {
	var candidates []string
	if envBin := strings.TrimSpace(os.Getenv("VELOCITY_CUDA_BIN")); envBin != "" {
		candidates = append(candidates, envBin)
	}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "velocity-cuda.exe"))
	}
	candidates = append(candidates, "velocity-cuda.exe", filepath.Join("..", "..", "..", "velocity-cuda.exe"))
	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			if abs, absErr := filepath.Abs(p); absErr == nil {
				return abs
			}
			return p
		}
	}
	return ""
}

// CanUseCudaColorRunner reports whether velocity-cuda.exe can execute this job zero-copy on GPU.
func CanUseCudaColorRunner(req model.ProcessRequest, probe ProbeInfo, speed float64) bool {
	if ResolveCudaRunnerPath() == "" {
		return false
	}
	layers := req.ResolveLayers()
	for _, l := range layers {
		lType := strings.ToLower(strings.TrimSpace(l.Type))
		if lType == "text" && strings.TrimSpace(l.RasterDataURL) == "" {
			return false
		}
	}
	color := strings.ToLower(strings.TrimSpace(req.VideoFilter))
	hasColor := color != "" && color != "none"
	hasTrim := false
	if req.TrimEnabled {
		var excludes []mathutil.Interval
		for _, seg := range req.TrimSegments {
			excludes = append(excludes, mathutil.Interval{Start: seg.Start, End: seg.End})
		}
		if kept, err := mathutil.ComputeKeepIntervals(probe.Duration, excludes); err == nil && mathutil.IsTrimEffective(kept, probe.Duration) {
			hasTrim = true
		}
	}
	return hasColor || len(layers) > 0 || hasTrim || math.Abs(speed-1.0) > 0.001
}

// BuildCudaExtraArgs builds --trim, --speed, --atempo, and --overlay flags for velocity-cuda.exe.
func BuildCudaExtraArgs(
	req model.ProcessRequest,
	probe ProbeInfo,
	uploadsDir string,
	outDur, speed float64,
) ([]string, []string, bool) {
	var args, temps []string
	if math.Abs(speed-1.0) > 0.001 {
		args = append(args, "--speed", fmt.Sprintf("%.6g", speed))
		if probe.HasAudio {
			atempo, err := mathutil.BuildAtempoChain(speed)
			if err != nil {
				return nil, nil, false
			}
			if atempo != "" {
				args = append(args, "--atempo", atempo)
			}
		}
	}
	if req.TrimEnabled {
		var excludes []mathutil.Interval
		for _, seg := range req.TrimSegments {
			excludes = append(excludes, mathutil.Interval{Start: seg.Start, End: seg.End})
		}
		kept, err := mathutil.ComputeKeepIntervals(probe.Duration, excludes)
		if err != nil {
			return nil, nil, false
		}
		if mathutil.IsTrimEffective(kept, probe.Duration) {
			var parts []string
			for _, seg := range kept {
				parts = append(parts, fmt.Sprintf("%.3f-%.3f", seg.Start, seg.End))
			}
			args = append(args, "--trim", strings.Join(parts, ","))
		}
	}
	for idx, layer := range req.ResolveLayers() {
		var imgPath string
		var err error
		if strings.TrimSpace(layer.RasterDataURL) != "" {
			imgPath, err = writeTempRasterPNG(idx+100, layer.RasterDataURL)
			if err == nil {
				temps = append(temps, imgPath)
			}
		} else {
			imgPath, err = resolveOverlayImagePath(uploadsDir, layer)
		}
		if err != nil {
			for _, t := range temps {
				_ = os.Remove(t)
			}
			return nil, nil, false
		}
		wRatio := layer.WidthRatio
		if wRatio <= 0 {
			wRatio = 0.2
		}
		targetW := mathutil.ComputeOverlayImageWidth(probe.Width, wRatio)
		opacity := mathutil.ClampFloat(mathutil.ResolveOptionalFloat(layer.Opacity, 100), 0, 100) / 100.0
		xRatio := mathutil.ClampFloat(mathutil.ResolveOptionalFloat(layer.XRatio, 0.5), 0, 1)
		yRatio := mathutil.ClampFloat(mathutil.ResolveOptionalFloat(layer.YRatio, 0.5), 0, 1)
		sPct := mathutil.ClampFloat(mathutil.ResolveOptionalFloat(layer.StartPct, 0), 0, 100)
		ePct := mathutil.ClampFloat(mathutil.ResolveOptionalFloat(layer.EndPct, 100), 0, 100)
		if ePct < sPct {
			ePct = sPct
		}
		t0 := math.Max(0, outDur*(sPct/100.0))
		t1 := 1e12
		if ePct < 99.95 {
			t1 = outDur * (ePct / 100.0)
		}
		spec := fmt.Sprintf("%s|%d|%.4f|%.4f|%.3f|%.3f|%.3f", imgPath, targetW, xRatio, yRatio, opacity, t0, t1)
		args = append(args, "--overlay", spec)
	}
	return args, temps, true
}

// RunCudaColorJob executes velocity-cuda.exe for 100% Zero-Copy NVDEC -> NVRTC Color+Overlay -> NVENC rendering.
func RunCudaColorJob(
	ctx context.Context,
	runnerPath, inputPath, outputPath string,
	totalTargetSecs float64,
	cfg EncoderConfig,
	hooks JobHooks,
) error {
	cudaPathOnce.Do(func() {
		cur := os.Getenv("PATH")
		extra := `D:\code\environment\ffmpeg-shared-9.0.2\bin;D:\code\environment\cuda-12.8\bin`
		if !strings.Contains(strings.ToLower(cur), "ffmpeg-shared-9.0.2") {
			_ = os.Setenv("PATH", extra+";"+cur)
		}
	})

	isHEVC := strings.EqualFold(cfg.Codec, "hevc") || strings.EqualFold(cfg.Codec, "h265")
	codec := "h264"
	encName := "h264_nvenc"
	if isHEVC {
		codec = "hevc"
		encName = "hevc_nvenc"
	}
	preset := strings.TrimSpace(cfg.Preset)
	if preset == "" {
		preset = "p4"
	}
	mode := model.NormalizeQualityMode(cfg.QualityMode)
	cq, rateArgs := resolveQualityAndCap(mode, isHEVC, cfg.SourceBitrateKbps, cfg.Speed)
	maxrate := "0"
	for i := 0; i+1 < len(rateArgs); i++ {
		if rateArgs[i] == "-maxrate" {
			k := strings.TrimSuffix(strings.TrimSpace(rateArgs[i+1]), "k")
			if _, err := strconv.Atoi(k); err == nil {
				maxrate = k
			}
		}
	}

	color := strings.ToLower(strings.TrimSpace(cfg.CudaColorFilter))
	if color == "" {
		color = "none"
	}
	if hooks.SetEncoder != nil {
		hooks.SetEncoder("GPU", fmt.Sprintf("%s (CUDA NVRTC Zero-Copy+NVENC, Preset: %s, %s)", encName, preset, mode))
	}
	args := []string{
		"--input", inputPath,
		"--output", outputPath,
		"--color", color,
		"--codec", codec,
		"--preset", preset,
		"--cq", cq,
		"--maxrate", maxrate,
	}
	args = append(args, cfg.CudaExtraArgs...)
	if hooks.Log != nil {
		hooks.Log("Executing [CUDA NVRTC Zero-Copy+NVENC]: " + runnerPath + " " + strings.Join(args, " "))
	}
	return RunFFmpegCommand(ctx, runnerPath, args, totalTargetSecs, 0, hooks)
}
