package server

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"velocityengine/internal/ffmpeg"
	"velocityengine/internal/model"
	"velocityengine/internal/syswin"
)

func (s *Server) handleCapabilities(w http.ResponseWriter, r *http.Request) {
	caps := s.cfg.Capabilities
	if !caps.FFmpegAvailable && !caps.FFprobeAvailable {
		caps = ffmpeg.DetectCapabilities(r.Context(), s.cfg.Bins)
	}
	writeJSON(w, http.StatusOK, caps)
}

func (s *Server) handleSelectFile(w http.ResponseWriter, r *http.Request) {
	var req model.SelectFileRequest
	_ = json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req)
	if req.FileType == "" {
		req.FileType = "video"
	}

	fn := s.cfg.SelectFileFn
	if fn == nil {
		fn = syswin.SelectFileDialog
	}
	selected, err := fn(r.Context(), req.FileType)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if selected == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"message": "Cancelled",
		})
		return
	}

	var sizeMB float64
	if info, statErr := os.Stat(selected); statErr == nil && !info.IsDir() {
		sizeMB = float64(int64((float64(info.Size())/(1024*1024))*100+0.5)) / 100
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"path":    selected,
		"name":    filepath.Base(selected),
		"size_mb": sizeMB,
	})
}

func (s *Server) handleProbe(w http.ResponseWriter, r *http.Request) {
	var req model.ProbeRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	rawPath := strings.TrimSpace(req.Path)
	if rawPath == "" {
		writeJSONError(w, http.StatusBadRequest, "missing path")
		return
	}
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid path")
		return
	}
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSONError(w, http.StatusNotFound, "file not found: "+absPath)
			return
		}
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if info.IsDir() {
		writeJSONError(w, http.StatusBadRequest, "path is a directory, expected media file")
		return
	}

	probe, err := ffmpeg.ProbeFile(r.Context(), s.cfg.Bins.FFprobe, absPath)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, probe.ProbeResponse)
}

func (s *Server) handlePreview(w http.ResponseWriter, r *http.Request) {
	rawPath := strings.TrimSpace(r.URL.Query().Get("path"))
	if rawPath == "" {
		writeJSONError(w, http.StatusBadRequest, "missing path query parameter")
		return
	}
	absPath, err := filepath.Abs(rawPath)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid path")
		return
	}
	info, err := os.Stat(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			writeJSONError(w, http.StatusNotFound, "file not found")
			return
		}
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	if info.IsDir() {
		writeJSONError(w, http.StatusBadRequest, "cannot preview a directory")
		return
	}

	if ct := mimeTypeForExt(filepath.Ext(absPath)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	http.ServeFile(w, r, absPath)
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	rawName := r.PathValue("filename")
	decoded, err := url.PathUnescape(rawName)
	if err != nil {
		decoded = rawName
	}
	cleanName := filepath.Base(strings.TrimSpace(decoded))
	if cleanName == "" || cleanName == "." || cleanName == ".." {
		writeJSONError(w, http.StatusBadRequest, "invalid filename")
		return
	}

	targetPath := filepath.Join(s.cfg.DownloadsDir, cleanName)
	rel, relErr := filepath.Rel(s.cfg.DownloadsDir, targetPath)
	if relErr != nil || strings.HasPrefix(rel, "..") {
		writeJSONError(w, http.StatusBadRequest, "invalid download path")
		return
	}

	info, err := os.Stat(targetPath)
	if err != nil || info.IsDir() {
		writeJSONError(w, http.StatusNotFound, "download file not found")
		return
	}

	if ct := mimeTypeForExt(filepath.Ext(targetPath)); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if dl := r.URL.Query().Get("download"); dl == "1" || strings.EqualFold(dl, "true") {
		w.Header().Set("Content-Disposition", formatRFC5987Attachment(cleanName))
	}
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	http.ServeFile(w, r, targetPath)
}

func (s *Server) handleShowExplorer(w http.ResponseWriter, r *http.Request) {
	var req model.ShowExplorerRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	cleanName := filepath.Base(strings.TrimSpace(req.Filename))
	if cleanName == "" || cleanName == "." || cleanName == ".." {
		writeJSONError(w, http.StatusBadRequest, "missing filename")
		return
	}
	targetPath, err := filepath.Abs(filepath.Join(s.cfg.DownloadsDir, cleanName))
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}
	info, err := os.Stat(targetPath)
	if err != nil || info.IsDir() {
		writeJSONError(w, http.StatusNotFound, "file not found in downloads")
		return
	}

	fn := s.cfg.ExplorerFn
	if fn == nil {
		fn = syswin.OpenInExplorer
	}
	if err := fn(targetPath); err != nil {
		writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
