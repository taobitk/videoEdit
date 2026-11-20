package ffmpeg

import (
	"context"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"

	"velocityengine/internal/mathutil"
)

// AreClipsCopyCompatible checks whether all probed clips share identical stream params for -c copy.
func AreClipsCopyCompatible(clips []ProbeInfo) (bool, string) {
	if len(clips) < 2 {
		return false, "fewer than 2 clips"
	}
	base := clips[0]
	if !isMP4CopyFriendlyCodec(base.VideoCodec, base.HasAudio, base.AudioCodec) {
		return false, fmt.Sprintf("codec %s/%s not stream-copy safe for mp4", base.VideoCodec, base.AudioCodec)
	}
	for i := 1; i < len(clips); i++ {
		c := clips[i]
		if c.VideoCodec != base.VideoCodec {
			return false, fmt.Sprintf("video codec mismatch (%s vs %s)", base.VideoCodec, c.VideoCodec)
		}
		if c.RawWidth != base.RawWidth || c.RawHeight != base.RawHeight || c.Rotation != base.Rotation {
			return false, fmt.Sprintf("resolution/rotation mismatch (%dx%d@%d vs %dx%d@%d)",
				base.RawWidth, base.RawHeight, base.Rotation, c.RawWidth, c.RawHeight, c.Rotation)
		}
		if c.PixFmt != base.PixFmt {
			return false, fmt.Sprintf("pixel format mismatch (%s vs %s)", base.PixFmt, c.PixFmt)
		}
		if base.FPS > 0 && c.FPS > 0 && math.Abs(base.FPS-c.FPS) > 0.05 {
			return false, fmt.Sprintf("fps mismatch (%.3f vs %.3f)", base.FPS, c.FPS)
		}
		if c.HasAudio != base.HasAudio {
			return false, "audio presence mismatch across clips"
		}
		if base.HasAudio {
			if c.AudioCodec != base.AudioCodec || c.AudioSampleRate != base.AudioSampleRate || c.AudioChannels != base.AudioChannels {
				return false, fmt.Sprintf("audio format mismatch (%s %dHz %dch vs %s %dHz %dch)",
					base.AudioCodec, base.AudioSampleRate, base.AudioChannels,
					c.AudioCodec, c.AudioSampleRate, c.AudioChannels)
			}
		}
	}
	return true, ""
}

func isMP4CopyFriendlyCodec(vCodec string, hasAudio bool, aCodec string) bool {
	v := strings.ToLower(vCodec)
	if v != "h264" && v != "hevc" {
		return false
	}
	if !hasAudio {
		return true
	}
	a := strings.ToLower(aCodec)
	return a == "aac" || a == "mp3" || a == "ac3"
}

// BuildMergeFiltergraph builds a normalizing filter_complex concat graph anchored on clips[0].
func BuildMergeFiltergraph(clips []ProbeInfo) (string, []string) {
	return buildMergeGraph(clips, false)
}

// BuildZeroCopyMergeFiltergraph builds a CUDA Zero-Copy normalizing concat graph using scale_cuda + pad_cuda.
func BuildZeroCopyMergeFiltergraph(clips []ProbeInfo) (string, []string) {
	return buildMergeGraph(clips, true)
}

func cudaTransposeForRotation(rot int) string {
	norm := ((rot % 360) + 360) % 360
	switch norm {
	case 270:
		return "transpose_cuda=dir=clock,"
	case 90:
		return "transpose_cuda=dir=cclock,"
	case 180:
		return "transpose_cuda=dir=clock,transpose_cuda=dir=clock,"
	default:
		return ""
	}
}

func buildMergeGraph(clips []ProbeInfo, zeroCopyCUDA bool) (string, []string) {
	targetW := mathutil.MakeEven(clips[0].Width, 2)
	targetH := mathutil.MakeEven(clips[0].Height, 2)
	targetFPS := clips[0].FPS
	if targetFPS <= 0 {
		targetFPS = 30
	}
	fpsStr := strconv.FormatFloat(math.Round(targetFPS*1000)/1000, 'f', -1, 64)

	anyAudio := false
	for _, c := range clips {
		if c.HasAudio {
			anyAudio = true
			break
		}
	}

	var parts []string
	var concatInputs strings.Builder
	for i, c := range clips {
		if zeroCopyCUDA {
			transpose := cudaTransposeForRotation(c.Rotation)
			parts = append(parts, fmt.Sprintf(
				"[%d:v]%sfps=%s,scale_cuda=w=%d:h=%d:force_original_aspect_ratio=decrease:format=yuv420p,pad_cuda=w=%d:h=%d:x=-1:y=-1:color=black,setsar=1,setpts=PTS-STARTPTS[v%d]",
				i, transpose, fpsStr, targetW, targetH, targetW, targetH, i,
			))
		} else {
			parts = append(parts, fmt.Sprintf(
				"[%d:v]scale=w=%d:h=%d:force_original_aspect_ratio=decrease,pad=%d:%d:(ow-iw)/2:(oh-ih)/2:black,setsar=1,fps=%s,format=yuv420p,setpts=PTS-STARTPTS[v%d]",
				i, targetW, targetH, targetW, targetH, fpsStr, i,
			))
		}
		concatInputs.WriteString(fmt.Sprintf("[v%d]", i))

		if anyAudio {
			if c.HasAudio {
				parts = append(parts, fmt.Sprintf(
					"[%d:a]aresample=48000,aformat=sample_fmts=fltp:channel_layouts=stereo,asetpts=PTS-STARTPTS[a%d]",
					i, i,
				))
			} else {
				dur := math.Max(0.05, c.Duration)
				parts = append(parts, fmt.Sprintf(
					"anullsrc=channel_layout=stereo:sample_rate=48000,atrim=duration=%.3f,asetpts=PTS-STARTPTS[a%d]",
					dur, i,
				))
			}
			concatInputs.WriteString(fmt.Sprintf("[a%d]", i))
		}
	}

	if anyAudio {
		parts = append(parts, fmt.Sprintf("%sconcat=n=%d:v=1:a=1[v][a]", concatInputs.String(), len(clips)))
		return strings.Join(parts, ";"), []string{"-map", "[v]", "-map", "[a]"}
	}
	parts = append(parts, fmt.Sprintf("%sconcat=n=%d:v=1:a=0[v]", concatInputs.String(), len(clips)))
	return strings.Join(parts, ";"), []string{"-map", "[v]"}
}

// RunMergeJob merges probed clips using stream copy if compatible, or normalized filtergraph re-encode.
func RunMergeJob(
	ctx context.Context,
	ffmpegPath string,
	clips []ProbeInfo,
	outputPath string,
	cfg EncoderConfig,
	hooks JobHooks,
	onStep func(step string),
) error {
	totalDur := 0.0
	for _, c := range clips {
		totalDur += c.Duration
	}

	compatible, reason := AreClipsCopyCompatible(clips)
	if compatible {
		if onStep != nil {
			onStep("Merging (stream copy)...")
		}
		if hooks.SetEncoder != nil {
			hooks.SetEncoder("COPY", "Concat Stream Copy (-c copy)")
		}
		if hooks.Log != nil {
			hooks.Log("All clips compatible; attempting fast concat stream copy...")
		}

		listFile, err := os.CreateTemp("", "ve_concat_*.txt")
		if err == nil {
			listPath := listFile.Name()
			for _, c := range clips {
				_, _ = listFile.WriteString(FormatConcatListLine(c.Path) + "\n")
			}
			_ = listFile.Close()
			defer os.Remove(listPath)

			args := []string{
				"-y", "-f", "concat", "-safe", "0", "-i", listPath,
				"-c", "copy", "-movflags", "+faststart", "-progress", "pipe:1", outputPath,
			}
			copyErr := RunFFmpegCommand(ctx, ffmpegPath, args, totalDur, 0, hooks)
			if copyErr == nil {
				if stat, stErr := os.Stat(outputPath); stErr == nil && stat.Size() >= 100 {
					return nil
				}
			}
			if ctx.Err() != nil {
				_ = os.Remove(outputPath)
				return copyErr
			}
			_ = os.Remove(outputPath)
			if hooks.Log != nil {
				hooks.Log("Stream copy fallback triggered; switching to normalized filtergraph re-encode...")
			}
			if hooks.ResetProgress != nil {
				hooks.ResetProgress()
			}
		}
	} else if hooks.Log != nil {
		hooks.Log("Re-encoding merge required (" + reason + ")")
	}

	if onStep != nil {
		onStep("Normalizing & merging clips...")
	}

	filterComplex, mapArgs := BuildMergeFiltergraph(clips)
	inputs := make([]VideoInput, len(clips))
	for i, c := range clips {
		in := VideoInput{Path: c.Path}
		if c.Rotation != 0 {
			in.ZeroCopyPreArgs = []string{"-noautorotate"}
		}
		inputs[i] = in
	}
	cudaFC, _ := BuildZeroCopyMergeFiltergraph(clips)
	cfg.ZeroCopyMidArgs = append([]string{"-filter_complex", cudaFC}, mapArgs...)
	midArgs := append([]string{"-filter_complex", filterComplex}, mapArgs...)

	return RunWith3TierFallback(ctx, ffmpegPath, inputs, nil, midArgs, outputPath, totalDur, 0, cfg, hooks)
}
