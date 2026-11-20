package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"velocityengine/internal/ffmpeg"
	"velocityengine/internal/mathutil"
	"velocityengine/internal/model"
)

func (s *Server) handleSplit(w http.ResponseWriter, r *http.Request) {
	if s.state.IsProcessing() {
		writeJSONError(w, http.StatusConflict, "another job is currently processing")
		return
	}

	var req model.SplitRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}

	codec, err := model.NormalizeCodec(req.Codec)
	if err != nil {
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

	segments, err := ffmpeg.ComputeSplitSegments(req, probe.Duration)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	useGPU := true
	if req.UseGPU != nil {
		useGPU = *req.UseGPU
	}

	jobCtx, ok := s.state.TryStartJob(s.rootCtx, "split", fmt.Sprintf("Splitting into %d segments...", len(segments)), "")
	if !ok {
		writeJSONError(w, http.StatusConflict, "another job is currently processing")
		return
	}

	cfg := ffmpeg.EncoderConfig{
		UseGPU:            useGPU,
		Codec:             codec,
		Preset:            req.Preset,
		QualityMode:       model.NormalizeQualityMode(req.QualityMode),
		CQ:                model.NormalizeCQForCodec(codec, req.CQ),
		SourceBitrateKbps: probe.BitrateKbps,
		Speed:             1.0,
	}
	go s.runSplitWorker(jobCtx, inputPath, segments, probe.Duration, req, cfg)

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "Split operation launched."})
}

func (s *Server) runSplitWorker(
	jobCtx context.Context,
	inputPath string,
	segments []mathutil.Interval,
	totalDuration float64,
	req model.SplitRequest,
	cfg ffmpeg.EncoderConfig,
) {
	outputs, err := ffmpeg.RunSplitJob(
		jobCtx,
		s.cfg.Bins.FFmpeg,
		inputPath,
		s.cfg.DownloadsDir,
		segments,
		totalDuration,
		req,
		cfg,
		s.state.Hooks(),
		func(idx, total int) {
			s.state.SetStep(fmt.Sprintf("Cutting segment %d/%d...", idx, total))
		},
	)
	if err != nil {
		s.state.FinishJob(err, nil, "", "", "")
		return
	}

	firstName, firstURL := "", ""
	if len(outputs) > 0 {
		firstName = outputs[0].Filename
		firstURL = outputs[0].URL
	}
	s.state.FinishJob(nil, outputs, firstName, firstURL, fmt.Sprintf("Complete! %d segments", len(outputs)))
}

func (s *Server) handleMerge(w http.ResponseWriter, r *http.Request) {
	if s.state.IsProcessing() {
		writeJSONError(w, http.StatusConflict, "another job is currently processing")
		return
	}

	var req model.MergeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON request body")
		return
	}
	if len(req.Files) < 2 {
		writeJSONError(w, http.StatusBadRequest, "at least 2 files are required for merge")
		return
	}

	codec, err := model.NormalizeCodec(req.Codec)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	var clips []ffmpeg.ProbeInfo
	for _, entry := range req.Files {
		inPath, statusCode, err := s.resolveValidMediaFile(entry.Type, entry.Filename, entry.Path, "")
		if err != nil {
			writeJSONError(w, statusCode, err.Error())
			return
		}
		probe, err := ffmpeg.ProbeFile(r.Context(), s.cfg.Bins.FFprobe, inPath)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, fmt.Sprintf("probe failed for %s: %v", filepath.Base(inPath), err))
			return
		}
		clips = append(clips, probe)
	}

	useGPU := true
	if req.UseGPU != nil {
		useGPU = *req.UseGPU
	}

	outName := fmt.Sprintf("merged_%d.mp4", time.Now().Unix())
	outPath := filepath.Join(s.cfg.DownloadsDir, outName)

	jobCtx, ok := s.state.TryStartJob(s.rootCtx, "merge", "Analyzing merge clips...", outName)
	if !ok {
		writeJSONError(w, http.StatusConflict, "another job is currently processing")
		return
	}

	srcBitrate := 0
	for _, c := range clips {
		if c.BitrateKbps > srcBitrate {
			srcBitrate = c.BitrateKbps
		}
	}
	cfg := ffmpeg.EncoderConfig{
		UseGPU:            useGPU,
		Codec:             codec,
		Preset:            req.Preset,
		QualityMode:       model.NormalizeQualityMode(req.QualityMode),
		CQ:                model.NormalizeCQForCodec(codec, req.CQ),
		SourceBitrateKbps: srcBitrate,
		Speed:             1.0,
	}
	go s.runMergeWorker(jobCtx, clips, outPath, outName, cfg)

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "Merge operation launched."})
}

func (s *Server) runMergeWorker(
	jobCtx context.Context,
	clips []ffmpeg.ProbeInfo,
	outPath, outName string,
	cfg ffmpeg.EncoderConfig,
) {
	err := ffmpeg.RunMergeJob(
		jobCtx,
		s.cfg.Bins.FFmpeg,
		clips,
		outPath,
		cfg,
		s.state.Hooks(),
		s.state.SetStep,
	)
	if err != nil {
		_ = os.Remove(outPath)
		s.state.FinishJob(err, nil, "", "", "")
		return
	}

	outURL := "/downloads/" + url.PathEscape(outName)
	files := []model.OutputFile{{Filename: outName, URL: outURL}}
	s.state.FinishJob(nil, files, outName, outURL, "Merge complete!")
}
