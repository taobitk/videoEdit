package ffmpeg

import (
	"context"
	"os"
	"strings"
)

const cpuFallbackWarning = "⚠️ Không phát hiện card đồ họa NVIDIA (hoặc CUDA chưa sẵn sàng)! Đã tự động chuyển sang render bằng CPU (chậm hơn)."

// EncoderConfig defines encoding options for FFmpeg render runs.
type EncoderConfig struct {
	UseGPU            bool
	Codec             string // "h264" or "hevc"
	Preset            string // e.g. "p4" or "medium"
	QualityMode       string // "auto_cap", "compact", "high"
	CQ                int    // 20..28 (default 20)
	SourceBitrateKbps int
	Speed             float64
	ZeroCopyMidArgs   []string
	ZeroCopyPadRight  int
	Tier1GlobalArgs   []string
	Tier1MidArgs      []string
	CudaColorFilter   string
	CudaExtraArgs     []string
}

// JobHooks lets the job runner update state, log output, and expose active PIDs for cancellation.
type JobHooks struct {
	SetPID         func(pid int)
	SetEncoder     func(activeEncoder, detail string)
	ResetProgress  func()
	UpdateProgress func(ProgressSnapshot)
	Log            func(line string)
	Warn           func(msg string)
}

type execTier int

const (
	tierCUDAAndNVENC execTier = iota + 1
	tierCPUDecodeNVENC
	tierCPUEncode
)

// VideoInput pairs an input media path with optional pre-input flags like -ss.
type VideoInput struct {
	Path            string
	PreArgs         []string
	ZeroCopyPreArgs []string
}

// RunWith3TierFallback runs FFmpeg with Tier 1 (CUDA+NVENC) -> Tier 2 (CPU decode+NVENC) -> Tier 3 (CPU).
func RunWith3TierFallback(
	ctx context.Context,
	ffmpegPath string,
	videoInputs []VideoInput,
	extraInputArgs []string,
	midArgs []string,
	outputPath string,
	totalTargetSecs float64,
	offsetSecs float64,
	cfg EncoderConfig,
	hooks JobHooks,
) error {
	if !cfg.UseGPU || strings.EqualFold(cfg.Codec, "av1") {
		return runTier(ctx, ffmpegPath, videoInputs, extraInputArgs, midArgs, outputPath, totalTargetSecs, offsetSecs, cfg, tierCPUEncode, hooks)
	}

	// Tier 1: CUDA decode + NVENC encode.
	err := runTier(ctx, ffmpegPath, videoInputs, extraInputArgs, midArgs, outputPath, totalTargetSecs, offsetSecs, cfg, tierCUDAAndNVENC, hooks)
	if err == nil || ctx.Err() != nil {
		return err
	}

	var stderrMsg string
	if e, ok := err.(*ffmpegExecError); ok {
		stderrMsg = e.stderr
	} else {
		return err
	}

	if IsCUDADecodeError(stderrMsg) || IsHardwareNVENCError(stderrMsg) {
		if hooks.Log != nil {
			hooks.Log("WARNING: CUDA hardware decode/NVENC attempt failed, retrying with CPU decode + NVENC...")
		}
		if hooks.ResetProgress != nil {
			hooks.ResetProgress()
		}
		_ = os.Remove(outputPath)

		err2 := runTier(ctx, ffmpegPath, videoInputs, extraInputArgs, midArgs, outputPath, totalTargetSecs, offsetSecs, cfg, tierCPUDecodeNVENC, hooks)
		if err2 == nil || ctx.Err() != nil {
			return err2
		}
		if e2, ok := err2.(*ffmpegExecError); ok {
			stderrMsg = e2.stderr
		} else {
			return err2
		}
	}

	if IsHardwareNVENCError(stderrMsg) || IsCUDADecodeError(stderrMsg) {
		if hooks.Log != nil {
			hooks.Log("WARNING: NVENC/CUDA unavailable; falling back to CPU software encoding...")
		}
		if hooks.Warn != nil {
			hooks.Warn(cpuFallbackWarning)
		}
		if hooks.ResetProgress != nil {
			hooks.ResetProgress()
		}
		_ = os.Remove(outputPath)
		return runTier(ctx, ffmpegPath, videoInputs, extraInputArgs, midArgs, outputPath, totalTargetSecs, offsetSecs, cfg, tierCPUEncode, hooks)
	}

	return err
}

func runTier(
	ctx context.Context,
	ffmpegPath string,
	videoInputs []VideoInput,
	extraInputArgs []string,
	midArgs []string,
	outputPath string,
	totalTargetSecs float64,
	offsetSecs float64,
	cfg EncoderConfig,
	tier execTier,
	hooks JobHooks,
) error {
	args := []string{"-y"}
	useZeroCopy := tier == tierCUDAAndNVENC && len(cfg.ZeroCopyMidArgs) > 0
	if tier == tierCUDAAndNVENC && !useZeroCopy && len(cfg.Tier1GlobalArgs) > 0 {
		args = append(args, cfg.Tier1GlobalArgs...)
	}
	for _, in := range videoInputs {
		args = append(args, in.PreArgs...)
		if useZeroCopy && len(in.ZeroCopyPreArgs) > 0 {
			args = append(args, in.ZeroCopyPreArgs...)
		}
		if tier == tierCUDAAndNVENC {
			args = append(args, "-hwaccel", "cuda")
			if useZeroCopy {
				args = append(args, "-hwaccel_output_format", "cuda")
			}
		}
		args = append(args, "-i", in.Path)
	}
	args = append(args, extraInputArgs...)
	if useZeroCopy {
		args = append(args, cfg.ZeroCopyMidArgs...)
	} else if tier == tierCUDAAndNVENC && len(cfg.Tier1MidArgs) > 0 {
		args = append(args, cfg.Tier1MidArgs...)
	} else {
		args = append(args, midArgs...)
	}

	encArgs, activeLabel, detail := encoderArgsForTier(cfg, tier)
	if hooks.SetEncoder != nil {
		hooks.SetEncoder(activeLabel, detail)
	}
	args = append(args, encArgs...)
	args = append(args, "-progress", "pipe:1", outputPath)

	if hooks.Log != nil {
		hooks.Log("Executing [" + detail + "]: " + ffmpegPath + " " + strings.Join(args, " "))
	}

	return RunFFmpegCommand(ctx, ffmpegPath, args, totalTargetSecs, offsetSecs, hooks)
}
