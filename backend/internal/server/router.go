package server

import (
	"context"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"

	"velocityengine/internal/ffmpeg"
	"velocityengine/internal/model"
)

// Config configures the VelocityEngine HTTP server.
type Config struct {
	WorkDir      string
	DownloadsDir string
	UploadsDir   string
	StaticFS     fs.FS
	Bins         ffmpeg.BinaryPaths
	Capabilities model.SystemCapabilities
	SelectFileFn func(ctx context.Context, fileType string) (string, error)
	ExplorerFn   func(absPath string) error
}

// Server serves VelocityEngine REST APIs, SSE stream, media files, and embedded SPA.
type Server struct {
	cfg      Config
	state    *StateManager
	rootCtx  context.Context
	mux      *http.ServeMux
}

// New creates a configured Server and ensures downloads/ and uploads/ directories exist.
func New(rootCtx context.Context, cfg Config) (*Server, error) {
	if cfg.WorkDir == "" {
		cwd, _ := os.Getwd()
		cfg.WorkDir = cwd
	}
	if cfg.DownloadsDir == "" {
		cfg.DownloadsDir = filepath.Join(cfg.WorkDir, "downloads")
	}
	if cfg.UploadsDir == "" {
		cfg.UploadsDir = filepath.Join(cfg.WorkDir, "uploads")
	}
	if err := os.MkdirAll(cfg.DownloadsDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.UploadsDir, 0755); err != nil {
		return nil, err
	}
	if cfg.Bins.FFmpeg == "" && cfg.Bins.FFprobe == "" {
		cfg.Bins = ffmpeg.ResolveBinaries()
	}

	s := &Server{
		cfg:     cfg,
		state:   NewStateManager(),
		rootCtx: rootCtx,
		mux:     http.NewServeMux(),
	}
	s.registerRoutes()
	return s, nil
}

// State returns the underlying StateManager for inspection or graceful shutdown.
func (s *Server) State() *StateManager {
	return s.state
}

// Handler returns the root http.Handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

func (s *Server) registerRoutes() {
	s.mux.HandleFunc("GET /api/system/capabilities", s.handleCapabilities)
	s.mux.HandleFunc("GET /api/status", s.handleStatus)
	s.mux.HandleFunc("GET /api/events", s.handleEvents)
	s.mux.HandleFunc("POST /api/select_file", s.handleSelectFile)
	s.mux.HandleFunc("POST /api/probe", s.handleProbe)
	s.mux.HandleFunc("GET /api/preview", s.handlePreview)
	s.mux.HandleFunc("GET /downloads/{filename}", s.handleDownload)
	s.mux.HandleFunc("POST /api/show_explorer", s.handleShowExplorer)
	s.mux.HandleFunc("POST /api/process", s.handleProcess)
	s.mux.HandleFunc("POST /api/split", s.handleSplit)
	s.mux.HandleFunc("POST /api/merge", s.handleMerge)
	s.mux.HandleFunc("POST /api/cancel", s.handleCancel)
	s.mux.HandleFunc("POST /api/reset", s.handleReset)
	s.mux.HandleFunc("GET /", s.handleStaticOrSPA)
}
