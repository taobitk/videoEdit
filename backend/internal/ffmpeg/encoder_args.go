package ffmpeg

import (
	"fmt"
	"strconv"
	"strings"

	"velocityengine/internal/model"
)

func mapSVTPreset(preset string) string {
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "p1", "10":
		return "10"
	case "p7", "6":
		return "6"
	case "4", "5", "7", "9", "11", "12":
		return strings.TrimSpace(preset)
	default:
		return "8"
	}
}

func encoderArgsForTier(cfg EncoderConfig, tier execTier) ([]string, string, string) {
	preset := strings.TrimSpace(cfg.Preset)
	if preset == "" {
		preset = "p4"
	}
	isAV1 := strings.EqualFold(cfg.Codec, "av1") || strings.EqualFold(cfg.Codec, "svtav1")
	isHEVC := !isAV1 && (strings.EqualFold(cfg.Codec, "hevc") || strings.EqualFold(cfg.Codec, "h265"))
	cq, rateArgs := resolveQualityAndCap(cfg.Codec, cfg.CQ)

	if isAV1 {
		svtPreset := mapSVTPreset(preset)
		args := []string{"-c:v", "libsvtav1", "-preset", svtPreset, "-crf", cq, "-pix_fmt", "yuv420p", "-c:a", "aac", "-movflags", "+faststart"}
		return args, "CPU", fmt.Sprintf("libsvtav1 (Preset: %s, CRF %s)", svtPreset, cq)
	}

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
		return args, "CPU", fmt.Sprintf("%s (Preset: %s, CQ %s)", enc, cpuPreset, cq)
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
	return args, "GPU", fmt.Sprintf("%s (%s, Preset: %s, CQ %s)", enc, hwMode, preset, cq)
}

func resolveQualityAndCap(codec string, rawCQ int) (string, []string) {
	return strconv.Itoa(model.NormalizeCQForCodec(codec, rawCQ)), nil
}
