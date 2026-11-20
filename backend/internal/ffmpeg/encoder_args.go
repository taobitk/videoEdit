package ffmpeg

import (
	"fmt"
	"strings"

	"velocityengine/internal/model"
)

func encoderArgsForTier(cfg EncoderConfig, tier execTier) ([]string, string, string) {
	preset := strings.TrimSpace(cfg.Preset)
	if preset == "" {
		preset = "p4"
	}
	isHEVC := strings.EqualFold(cfg.Codec, "hevc") || strings.EqualFold(cfg.Codec, "h265")
	mode := model.NormalizeQualityMode(cfg.QualityMode)
	cq, rateArgs := resolveQualityAndCap(mode, isHEVC, cfg.SourceBitrateKbps, cfg.Speed)

	if tier == tierCPUEncode {
		cpuPreset := MapCPUPreset(preset)
		enc := "libx264"
		if isHEVC {
			enc = "libx265"
		}
		args := []string{"-c:v", enc, "-preset", cpuPreset, "-crf", cq}
		args = append(args, rateArgs...)
		args = append(args, "-pix_fmt", "yuv420p")
		if isHEVC {
			args = append(args, "-tag:v", "hvc1")
		}
		args = append(args, "-c:a", "aac", "-movflags", "+faststart")
		return args, "CPU", fmt.Sprintf("%s (Preset: %s, %s)", enc, cpuPreset, mode)
	}

	enc := "h264_nvenc"
	if isHEVC {
		enc = "hevc_nvenc"
	}
	args := []string{"-c:v", enc, "-preset", preset, "-rc", "vbr", "-b:v", "0", "-cq", cq, "-spatial_aq", "1"}
	args = append(args, rateArgs...)
	useZeroCopy := tier == tierCUDAAndNVENC && len(cfg.ZeroCopyMidArgs) > 0
	if !useZeroCopy {
		args = append(args, "-pix_fmt", "yuv420p")
	}
	if isHEVC {
		args = append(args, "-tag:v", "hvc1")
	}
	if useZeroCopy && cfg.ZeroCopyPadRight > 0 {
		if isHEVC {
			args = append(args, "-bsf:v", fmt.Sprintf("hevc_metadata=crop_right=%d", cfg.ZeroCopyPadRight))
		} else {
			args = append(args, "-bsf:v", fmt.Sprintf("h264_metadata=crop_right=%d", cfg.ZeroCopyPadRight))
		}
	}
	args = append(args, "-c:a", "aac", "-movflags", "+faststart")

	hwMode := "NVENC"
	if useZeroCopy {
		hwMode = "CUDA Zero-Copy+NVENC"
	} else if tier == tierCUDAAndNVENC && len(cfg.Tier1MidArgs) > 0 {
		hwMode = "Vulkan GPU Color+NVENC"
	} else if tier == tierCUDAAndNVENC {
		hwMode = "CUDA+NVENC"
	}
	return args, "GPU", fmt.Sprintf("%s (%s, Preset: %s, %s)", enc, hwMode, preset, mode)
}

func resolveQualityAndCap(_ string, _ bool, _ int, _ float64) (string, []string) {
	return "20", nil
}
