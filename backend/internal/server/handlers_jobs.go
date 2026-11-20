package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"velocityengine/internal/ffmpeg"
	"velocityengine/internal/mathutil"
	"velocityengine/internal/model"
)

func (s *Server) handleProcess(w http.ResponseWriter, r *http.Request) {
	if s.state.IsProcessing() {
		writeJSONError(w, http.StatusConflict, "another job is currently processing")
		return
	}

	var req model.ProcessRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<20)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	speed := 1.0
	if req.Speed != nil {
		speed = *req.Speed
	}
	if err := mathutil.ValidateSpeed(speed); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	codec, err := model.NormalizeCodec(req.Codec)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if _, err := ffmpeg.ColorFilterExpr(req.VideoFilter); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	inputPath, statusCode, err := s.resolveValidMediaFile(req.SourceType, req.Filename, req.LocalPath, req.Path)
	if err != nil {
		writeJSONError(w, statusCode, err.Error())
		return
	}

	probe, err := ffmpeg.ProbeFile(r.Context(), s.cfg.Bins.FFprobe, inputPath)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	plan, err := ffmpeg.BuildProcessFiltergraph(req, probe, s.cfg.UploadsDir)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	useGPU := true
	if req.UseGPU != nil {
		useGPU = *req.UseGPU
	}

	outName := buildProcessOutputName(inputPath, speed)
	outPath := filepath.Join(s.cfg.DownloadsDir, outName)
	if strings.EqualFold(filepath.Clean(outPath), filepath.Clean(inputPath)) {
		outName = fmt.Sprintf("%d_%s", time.Now().Unix(), outName)
		outPath = filepath.Join(s.cfg.DownloadsDir, outName)
	}

	jobCtx, ok := s.state.TryStartJob(s.rootCtx, "process", "Initializing render pipeline...", outName)
	if !ok {
		plan.Cleanup()
		writeJSONError(w, http.StatusConflict, "another job is currently processing")
		return
	}

	var zeroCopyMidArgs []string
	if plan.ZeroCopyCUDAFilterComplex != "" {
		zeroCopyMidArgs = append([]string{"-filter_complex", plan.ZeroCopyCUDAFilterComplex}, plan.MapArgs...)
	}
	var tier1MidArgs []string
	if plan.GPUColorFilterComplex != "" {
		tier1MidArgs = append([]string{"-filter_complex", plan.GPUColorFilterComplex}, plan.MapArgs...)
	}
	cudaColorFilter := ""
	var cudaExtraArgs []string
	if useGPU && ffmpeg.CanUseCudaColorRunner(req, probe, speed) {
		if extra, temps, ok := ffmpeg.BuildCudaExtraArgs(req, probe, s.cfg.UploadsDir, plan.OutputDuration, speed); ok {
			cudaColorFilter = req.VideoFilter
			if strings.TrimSpace(cudaColorFilter) == "" {
				cudaColorFilter = "none"
			}
			cudaExtraArgs = extra
			plan.TempFiles = append(plan.TempFiles, temps...)
		}
	}
	cfg := ffmpeg.EncoderConfig{
		UseGPU:            useGPU,
		Codec:             codec,
		Preset:            req.Preset,
		QualityMode:       model.NormalizeQualityMode(req.QualityMode),
		SourceBitrateKbps: probe.BitrateKbps,
		Speed:             speed,
		ZeroCopyMidArgs:   zeroCopyMidArgs,
		ZeroCopyPadRight:  plan.ZeroCopyPadRight,
		Tier1GlobalArgs:   plan.GPUColorGlobalArgs,
		Tier1MidArgs:      tier1MidArgs,
		CudaColorFilter:   cudaColorFilter,
		CudaExtraArgs:     cudaExtraArgs,
	}
	go s.runProcessWorker(jobCtx, inputPath, outPath, outName, plan, cfg)

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "Render engine launched."})
}

func (s *Server) runProcessWorker(
	jobCtx context.Context,
	inputPath, outPath, outName string,
	plan *ffmpeg.ProcessFilterPlan,
	cfg ffmpeg.EncoderConfig,
) {
	defer plan.Cleanup()
	s.state.SetStep("Rendering video...")

	midArgs := append([]string{"-filter_complex", plan.FilterComplex}, plan.MapArgs...)
	var err error
	if cfg.UseGPU && cfg.CudaColorFilter != "" {
		if runnerPath := ffmpeg.ResolveCudaRunnerPath(); runnerPath != "" {
			err = ffmpeg.RunCudaColorJob(jobCtx, runnerPath, inputPath, outPath, plan.OutputDuration, cfg, s.state.Hooks())
			if err != nil && jobCtx.Err() == nil {
				s.state.Hooks().Log(fmt.Sprintf("WARNING: CUDA NVRTC runner failed (%v), falling back to Vulkan/FFmpeg pipeline...", err))
				s.state.Hooks().ResetProgress()
				_ = os.Remove(outPath)
			}
		}
	}
	if !cfg.UseGPU || cfg.CudaColorFilter == "" || (err != nil && jobCtx.Err() == nil) {
		err = ffmpeg.RunWith3TierFallback(
			jobCtx,
			s.cfg.Bins.FFmpeg,
			[]ffmpeg.VideoInput{{Path: inputPath}},
			plan.ExtraInputArgs,
			midArgs,
			outPath,
			plan.OutputDuration,
			0,
			cfg,
			s.state.Hooks(),
		)
	}
	if err != nil {
		_ = os.Remove(outPath)
		s.state.FinishJob(err, nil, "", "", "")
		return
	}

	outURL := "/downloads/" + url.PathEscape(outName)
	files := []model.OutputFile{{Filename: outName, URL: outURL}}
	s.state.FinishJob(nil, files, outName, outURL, "Complete!")
}

func (s *Server) handleCancel(w http.ResponseWriter, _ *http.Request) {
	cancelled := s.state.CancelActiveJob()
	msg := "No active job."
	if cancelled {
		msg = "Cancellation requested."
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": msg})
}

func (s *Server) handleReset(w http.ResponseWriter, _ *http.Request) {
	if err := s.state.Reset(); err != nil {
		writeJSONError(w, http.StatusConflict, "cannot reset while rendering is in progress")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) resolveValidMediaFile(sourceType, filename, localPath, pathAlias string) (string, int, error) {
	p, err := model.ResolveInputPath(s.cfg.UploadsDir, sourceType, filename, localPath, pathAlias)
	if err != nil {
		return "", http.StatusBadRequest, err
	}
	info, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return "", http.StatusNotFound, fmt.Errorf("file not found: %s", p)
		}
		return "", http.StatusBadRequest, err
	}
	if info.IsDir() {
		return "", http.StatusBadRequest, fmt.Errorf("path is a directory: %s", p)
	}
	return p, http.StatusOK, nil
}

func buildProcessOutputName(inputPath string, speed float64) string {
	base := strings.TrimSuffix(filepath.Base(inputPath), filepath.Ext(inputPath))
	if base == "" {
		base = "output"
	}
	spdStr := strconv.FormatFloat(speed, 'f', -1, 64)
	return fmt.Sprintf("%s_%sx.mp4", base, spdStr)
}
