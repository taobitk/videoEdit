package ffmpeg

import (
	"fmt"
	"os"
	"strings"

	"velocityengine/internal/mathutil"
	"velocityengine/internal/model"
)

// ProcessFilterPlan holds the assembled FFmpeg filtergraph, extra inputs, maps, and temp files.
type ProcessFilterPlan struct {
	ExtraInputArgs            []string
	FilterComplex             string
	ZeroCopyCUDAFilterComplex string
	ZeroCopyPadRight          int
	GPUColorFilterComplex     string
	GPUColorGlobalArgs        []string
	MapArgs                   []string
	TempFiles                 []string
	KeptDuration              float64
	OutputDuration            float64
}

// Cleanup removes any temporary text files created for drawtext layers.
func (p *ProcessFilterPlan) Cleanup() {
	for _, f := range p.TempFiles {
		if f != "" {
			_ = os.Remove(f)
		}
	}
}

// BuildProcessFiltergraph constructs trim, speed, color, and Z-ordered layer filters.
func BuildProcessFiltergraph(req model.ProcessRequest, probe ProbeInfo, uploadsDir string) (*ProcessFilterPlan, error) {
	speed := 1.0
	if req.Speed != nil {
		speed = *req.Speed
	}
	if err := mathutil.ValidateSpeed(speed); err != nil {
		return nil, err
	}

	var excludes []mathutil.Interval
	if req.TrimEnabled {
		for _, seg := range req.TrimSegments {
			excludes = append(excludes, mathutil.Interval{Start: seg.Start, End: seg.End})
		}
	}
	kept, err := mathutil.ComputeKeepIntervals(probe.Duration, excludes)
	if err != nil {
		return nil, err
	}
	keptDur := mathutil.TotalDuration(kept)
	outDur := keptDur / speed

	colorExpr, err := ColorFilterExpr(req.VideoFilter)
	if err != nil {
		return nil, err
	}

	plan := &ProcessFilterPlan{
		KeptDuration:   keptDur,
		OutputDuration: outDur,
	}

	var steps []string
	var cudaSteps []string
	canZeroCopyCUDA := colorExpr == ""
	vTag, aTag := "[0:v]", "[0:a]"
	if req.TrimEnabled && mathutil.IsTrimEffective(kept, probe.Duration) {
		var trimParts []string
		var concatIn strings.Builder
		for idx, seg := range kept {
			trimParts = append(trimParts, fmt.Sprintf("[0:v]trim=start=%.3f:end=%.3f,setpts=PTS-STARTPTS[vt_%d]", seg.Start, seg.End, idx))
			concatIn.WriteString(fmt.Sprintf("[vt_%d]", idx))
			if probe.HasAudio {
				trimParts = append(trimParts, fmt.Sprintf("[0:a]atrim=start=%.3f:end=%.3f,asetpts=PTS-STARTPTS[at_%d]", seg.Start, seg.End, idx))
				concatIn.WriteString(fmt.Sprintf("[at_%d]", idx))
			}
		}
		aCount := 0
		concatOut := "[vt]"
		if probe.HasAudio {
			aCount = 1
			concatOut = "[vt][at]"
		}
		trimStep := strings.Join(trimParts, ";")
		concatStep := fmt.Sprintf("%sconcat=n=%d:v=1:a=%d%s", concatIn.String(), len(kept), aCount, concatOut)
		steps = append(steps, trimStep, concatStep)
		if canZeroCopyCUDA {
			cudaSteps = append(cudaSteps, trimStep, concatStep)
		}
		vTag, aTag = "[vt]", "[at]"
	}

	baseFilters := []string{fmt.Sprintf("setpts=(PTS-STARTPTS)/%.6g", speed)}
	if colorExpr != "" {
		baseFilters = append(baseFilters, colorExpr)
	}
	steps = append(steps, fmt.Sprintf("%s%s[v_base]", vTag, strings.Join(baseFilters, ",")))
	curV := "[v_base]"
	evenW := (probe.Width / 2) * 2
	evenH := (probe.Height / 2) * 2
	if evenW <= 0 || evenH <= 0 {
		canZeroCopyCUDA = false
	}
	layers := req.ResolveLayers()
	alignedW := ((evenW + 127) / 128) * 128
	padRight := alignedW - evenW
	if canZeroCopyCUDA {
		if len(layers) > 0 && padRight > 0 {
			cudaSteps = append(cudaSteps, fmt.Sprintf("%ssetpts=(PTS-STARTPTS)/%.6g,scale_cuda=w=%d:h=%d:format=yuv420p,pad_cuda=w=%d:h=%d:x=0:y=0[v_base]", vTag, speed, evenW, evenH, alignedW, evenH))
			plan.ZeroCopyPadRight = padRight
		} else {
			cudaSteps = append(cudaSteps, fmt.Sprintf("%ssetpts=(PTS-STARTPTS)/%.6g,scale_cuda=w=%d:h=%d:format=yuv420p[v_base]", vTag, speed, evenW, evenH))
		}
	}

	nextInputIdx := 1
	for idx, layer := range layers {
		lType := strings.ToLower(strings.TrimSpace(layer.Type))
		if strings.TrimSpace(layer.RasterDataURL) != "" {
			lType = "image"
		}
		switch lType {
		case "text":
			canZeroCopyCUDA = false
			txtPath, err := writeTempTextFile(idx, layer.Text)
			if err != nil {
				plan.Cleanup()
				return nil, err
			}
			if txtPath == "" {
				continue
			}
			plan.TempFiles = append(plan.TempFiles, txtPath)
			drawExpr := buildDrawtextStep(layer, txtPath, probe.Height, outDur)
			nextV := fmt.Sprintf("[v_l%d]", idx)
			steps = append(steps, fmt.Sprintf("%s%s%s", curV, drawExpr, nextV))
			curV = nextV

		case "image":
			var (
				imgPath string
				err     error
			)
			if strings.TrimSpace(layer.RasterDataURL) != "" {
				imgPath, err = writeTempRasterPNG(idx, layer.RasterDataURL)
				if err == nil {
					plan.TempFiles = append(plan.TempFiles, imgPath)
				}
			} else {
				imgPath, err = resolveOverlayImagePath(uploadsDir, layer)
			}
			if err != nil {
				plan.Cleanup()
				return nil, err
			}
			plan.ExtraInputArgs = append(plan.ExtraInputArgs, imageInputArgs(imgPath)...)
			imgIdx := nextInputIdx
			nextInputIdx++

			wRatio := layer.WidthRatio
			if wRatio <= 0 {
				wRatio = 0.2
			}
			targetW := mathutil.ComputeOverlayImageWidth(probe.Width, wRatio)
			opacity := mathutil.ClampFloat(mathutil.ResolveOptionalFloat(layer.Opacity, 100), 0, 100) / 100.0
			xRatio := mathutil.ClampFloat(mathutil.ResolveOptionalFloat(layer.XRatio, 0.5), 0, 1)
			yRatio := mathutil.ClampFloat(mathutil.ResolveOptionalFloat(layer.YRatio, 0.5), 0, 1)
			startPct := mathutil.ResolveOptionalFloat(layer.StartPct, 0)
			endPct := mathutil.ResolveOptionalFloat(layer.EndPct, 100)
			enableExpr := mathutil.ComputeOverlayEnable(outDur, startPct, endPct)

			imgTag := fmt.Sprintf("[img_%d]", idx)
			nextV := fmt.Sprintf("[v_l%d]", idx)
			steps = append(steps, fmt.Sprintf("[%d:v]format=rgba,scale=%d:-1,colorchannelmixer=aa=%.3f%s", imgIdx, targetW, opacity, imgTag))
			steps = append(steps, fmt.Sprintf("%s%soverlay=x=W*%.4f-overlay_w/2:y=H*%.4f-overlay_h/2:shortest=1:%s%s", curV, imgTag, xRatio, yRatio, enableExpr, nextV))
			if canZeroCopyCUDA {
				cudaSteps = append(cudaSteps, fmt.Sprintf("[%d:v]format=rgba,scale=%d:-1,colorchannelmixer=aa=%.3f,format=yuva420p,hwupload_cuda%s", imgIdx, targetW, opacity, imgTag))
				cudaSteps = append(cudaSteps, fmt.Sprintf("%s%soverlay_cuda=x=%d*%.4f-overlay_w/2:y=%d*%.4f-overlay_h/2:shortest=1:%s%s", curV, imgTag, evenW, xRatio, evenH, yRatio, enableExpr, nextV))
			}
			curV = nextV
		}
	}

	steps = append(steps, fmt.Sprintf("%sscale=trunc(iw/2)*2:trunc(ih/2)*2[v]", curV))
	if canZeroCopyCUDA {
		lastIdx := len(cudaSteps) - 1
		cudaSteps[lastIdx] = strings.TrimSuffix(cudaSteps[lastIdx], curV) + "[v]"
	}

	if probe.HasAudio {
		atempo, err := mathutil.BuildAtempoChain(speed)
		if err != nil {
			plan.Cleanup()
			return nil, err
		}
		audioStep := fmt.Sprintf("%sasetpts=PTS-STARTPTS[a]", aTag)
		if atempo != "" {
			audioStep = fmt.Sprintf("%s%s,asetpts=PTS-STARTPTS[a]", aTag, atempo)
		}
		steps = append(steps, audioStep)
		if canZeroCopyCUDA {
			cudaSteps = append(cudaSteps, audioStep)
		}
		plan.MapArgs = []string{"-map", "[v]", "-map", "[a]"}
	} else {
		plan.MapArgs = []string{"-map", "[v]"}
	}

	plan.FilterComplex = strings.Join(steps, ";")
	if canZeroCopyCUDA {
		plan.ZeroCopyCUDAFilterComplex = strings.Join(cudaSteps, ";")
	} else if colorExpr != "" {
		gpuColorExpr, shaderTmp, gpuGlobal, gpuErr := BuildGPUColorFilter(req.VideoFilter)
		if gpuErr == nil && gpuColorExpr != "" {
			if shaderTmp != "" {
				plan.TempFiles = append(plan.TempFiles, shaderTmp)
			}
			plan.GPUColorGlobalArgs = gpuGlobal
			plan.GPUColorFilterComplex = strings.Replace(plan.FilterComplex, colorExpr, gpuColorExpr, 1)
		}
	}
	return plan, nil
}
