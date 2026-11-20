package ffmpeg

import (
	"context"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"velocityengine/internal/mathutil"
	"velocityengine/internal/model"
)

// ComputeSplitSegments validates split parameters and returns ordered segment bounds.
func ComputeSplitSegments(req model.SplitRequest, duration float64) ([]mathutil.Interval, error) {
	mode := strings.ToLower(strings.TrimSpace(req.Mode))
	if mode == "" {
		if len(req.SplitPoints) > 0 {
			mode = "markers"
		} else {
			mode = "uniform"
		}
	}

	switch mode {
	case "markers":
		return mathutil.NormalizeSplitPoints(req.SplitPoints, duration)
	case "uniform":
		return mathutil.UniformSplitBounds(duration, req.SegmentDuration, req.Parts)
	default:
		return nil, fmt.Errorf("unsupported split mode %q (expected markers or uniform)", req.Mode)
	}
}

// RunSplitJob splits inputPath into segments using fast stream copy or frame-accurate re-encode.
func RunSplitJob(
	ctx context.Context,
	ffmpegPath string,
	inputPath string,
	downloadsDir string,
	segments []mathutil.Interval,
	totalDuration float64,
	req model.SplitRequest,
	cfg EncoderConfig,
	hooks JobHooks,
	onSegmentStart func(idx, total int),
) ([]model.OutputFile, error) {
	baseName := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	if baseName == "" {
		baseName = "split_output"
	}
	inExt := filepath.Ext(inputPath)
	if inExt == "" || req.Accurate {
		inExt = ".mp4"
	}

	var outputs []model.OutputFile
	var createdPaths []string
	startedAt := time.Now()
	offsetSecs := 0.0

	cleanupCreated := func() {
		for _, p := range createdPaths {
			_ = os.Remove(p)
		}
	}

	for i, seg := range segments {
		if ctx.Err() != nil {
			cleanupCreated()
			return nil, fmt.Errorf("split cancelled by user")
		}
		if onSegmentStart != nil {
			onSegmentStart(i+1, len(segments))
		}

		fname := fmt.Sprintf("%s_part%03d%s", baseName, i, inExt)
		outPath := filepath.Join(downloadsDir, fname)
		if hooks.Log != nil {
			hooks.Log(fmt.Sprintf("Segment %d/%d: %.2fs -> %.2fs => %s", i+1, len(segments), seg.Start, seg.End, fname))
		}

		segLen := seg.Duration()
		var err error
		if req.Accurate {
			var preArgs []string
			if seg.Start > 0.001 {
				preArgs = []string{"-ss", fmt.Sprintf("%.3f", seg.Start)}
			}
			midArgs := []string{
				"-t", fmt.Sprintf("%.3f", segLen),
				"-vf", "scale=trunc(iw/2)*2:trunc(ih/2)*2",
			}
			segCfg := cfg
			segCfg.ZeroCopyMidArgs = []string{
				"-t", fmt.Sprintf("%.3f", segLen),
				"-vf", "scale_cuda=w=trunc(iw/2)*2:h=trunc(ih/2)*2:format=yuv420p",
			}
			err = RunWith3TierFallback(
				ctx,
				ffmpegPath,
				[]VideoInput{{Path: inputPath, PreArgs: preArgs}},
				nil,
				midArgs,
				outPath,
				totalDuration,
				offsetSecs,
				segCfg,
				hooks,
			)
		} else {
			if hooks.SetEncoder != nil {
				hooks.SetEncoder("COPY", "Stream Copy (-c copy)")
			}
			args := []string{"-y"}
			if seg.Start > 0.001 {
				args = append(args, "-ss", fmt.Sprintf("%.3f", seg.Start))
			}
			args = append(args,
				"-i", inputPath,
				"-t", fmt.Sprintf("%.3f", segLen),
				"-c", "copy",
				"-avoid_negative_ts", "make_zero",
				"-progress", "pipe:1",
				outPath,
			)
			if hooks.Log != nil {
				hooks.Log("Executing [Stream Copy]: " + ffmpegPath + " " + strings.Join(args, " "))
			}
			err = RunFFmpegCommand(ctx, ffmpegPath, args, totalDuration, offsetSecs, hooks)
		}

		if err != nil {
			_ = os.Remove(outPath)
			cleanupCreated()
			return nil, fmt.Errorf("segment %d/%d failed: %w", i+1, len(segments), err)
		}

		stat, statErr := os.Stat(outPath)
		if statErr != nil || stat.Size() == 0 {
			_ = os.Remove(outPath)
			cleanupCreated()
			return nil, fmt.Errorf("segment %d/%d produced missing or empty output", i+1, len(segments))
		}

		createdPaths = append(createdPaths, outPath)
		outputs = append(outputs, model.OutputFile{
			Filename: fname,
			URL:      "/downloads/" + url.PathEscape(fname),
		})

		offsetSecs += segLen
		if hooks.UpdateProgress != nil && totalDuration > 0 {
			pct := math.Min(99.9, math.Round((offsetSecs/totalDuration)*1000)/10)
			elapsed := time.Since(startedAt).Seconds()
			eta := "Calculating..."
			if offsetSecs > 0.05 && elapsed > 0.05 {
				speedFactor := offsetSecs / elapsed
				if speedFactor > 0 {
					eta = FormatETA((totalDuration - offsetSecs) / speedFactor)
				}
			}
			hooks.UpdateProgress(ProgressSnapshot{
				Progress:      pct,
				Speed:         "fast",
				TimeProcessed: FormatClock(offsetSecs),
				ETA:           eta,
			})
		}
	}

	return outputs, nil
}
