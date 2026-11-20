package server

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func writeJSON(w http.ResponseWriter, code int, data any) {
	payload, err := json.Marshal(data)
	if err != nil {
		http.Error(w, `{"error":"json encoding failed"}`, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(code)
	_, _ = w.Write(payload)
}

func writeJSONError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]any{
		"success": false,
		"error":   msg,
	})
}

func (s *Server) handleStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, s.state.Snapshot())
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSONError(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	subCh, unsub := s.state.Subscribe()
	defer unsub()

	sendSnapshot := func() bool {
		snap := s.state.Snapshot()
		data, err := json.Marshal(snap)
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	if !sendSnapshot() {
		return
	}

	const minInterval = 100 * time.Millisecond
	lastSent := time.Now()
	var pendingTimer *time.Timer
	var pendingC <-chan time.Time

	stopPending := func() {
		if pendingTimer != nil {
			if !pendingTimer.Stop() {
				select {
				case <-pendingTimer.C:
				default:
				}
			}
			pendingTimer = nil
			pendingC = nil
		}
	}
	defer stopPending()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-pendingC:
			stopPending()
			lastSent = time.Now()
			if !sendSnapshot() {
				return
			}
		case <-subCh:
			snap := s.state.Snapshot()
			elapsed := time.Since(lastSent)
			if snap.Status != "processing" || elapsed >= minInterval {
				stopPending()
				lastSent = time.Now()
				if !sendSnapshot() {
					return
				}
			} else if pendingTimer == nil {
				pendingTimer = time.NewTimer(minInterval - elapsed)
				pendingC = pendingTimer.C
			}
		}
	}
}

func (s *Server) handleStaticOrSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/downloads/") {
		writeJSONError(w, http.StatusNotFound, "not found")
		return
	}
	if s.cfg.StaticFS == nil {
		writeJSONError(w, http.StatusNotFound, "frontend assets unavailable")
		return
	}

	cleanPath := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if cleanPath == "" || cleanPath == "." {
		cleanPath = "index.html"
	}

	if f, err := s.cfg.StaticFS.Open(cleanPath); err == nil {
		info, statErr := f.Stat()
		if statErr == nil && !info.IsDir() {
			defer f.Close()
			serveStaticEntry(w, cleanPath, f)
			return
		}
		_ = f.Close()
	}

	// SPA client-side route fallback.
	indexFile, err := s.cfg.StaticFS.Open("index.html")
	if err != nil {
		writeJSONError(w, http.StatusNotFound, "index.html not found")
		return
	}
	defer indexFile.Close()
	serveStaticEntry(w, "index.html", indexFile)
}

func serveStaticEntry(w http.ResponseWriter, name string, f fs.File) {
	ext := strings.ToLower(filepath.Ext(name))
	if ct := mimeTypeForExt(ext); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else if ct := mime.TypeByExtension(ext); ct != "" {
		w.Header().Set("Content-Type", ct)
	}
	if strings.HasPrefix(name, "_app/immutable/") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	_, _ = io.Copy(w, f)
}
