package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"velocityengine/internal/ffmpeg"
	"velocityengine/internal/server"
	"velocityengine/internal/syswin"
)

const (
	defaultPortStart = 8080
	defaultPortEnd   = 8099
)

func main() {
	portFlag := flag.Int("port", 0, "TCP port to listen on (overrides PORT env and 8080..8099 scan)")
	openFlag := flag.Bool("open", true, "Open browser automatically on launch")
	trayFlag := flag.Bool("tray", true, "Show system tray icon (Windows)")
	dirFlag := flag.String("dir", "", "Working directory for downloads and uploads")
	flag.Parse()

	workDir := resolveWorkDir(*dirFlag)
	bins := ffmpeg.ResolveBinaries()
	rootCtx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	caps := ffmpeg.DetectCapabilities(rootCtx, bins)
	staticFS, err := getEmbeddedDistFS()
	if err != nil {
		log.Fatalf("embedded dist error: %v", err)
	}

	srv, err := server.New(rootCtx, server.Config{
		WorkDir:      workDir,
		StaticFS:     staticFS,
		Bins:         bins,
		Capabilities: caps,
	})
	if err != nil {
		log.Fatalf("server init failed: %v", err)
	}

	ln, boundPort, err := bindListener(*portFlag)
	if err != nil {
		log.Fatalf("failed to bind server port: %v", err)
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/", boundPort)
	log.Printf("VelocityEngine v2 listening on %s (workDir=%s, GPU=%v %s)", url, workDir, caps.HasNvidiaGPU, caps.GPUName)

	httpSrv := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 15 * time.Second,
	}

	if *openFlag {
		go func() {
			time.Sleep(350 * time.Millisecond)
			_ = syswin.OpenBrowser(url)
		}()
	}
	if *trayFlag {
		go syswin.RunTray(
			rootCtx,
			fmt.Sprintf("VelocityEngine v2 (%s)", url),
			func() { _ = syswin.OpenBrowser(url) },
			stopSignals,
		)
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- httpSrv.Serve(ln)
	}()

	select {
	case <-rootCtx.Done():
		log.Println("Shutting down VelocityEngine...")
		srv.State().CancelActiveJob()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutCtx)
	case serveErr := <-errCh:
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", serveErr)
		}
	}
}

func bindListener(flagPort int) (net.Listener, int, error) {
	if flagPort > 0 {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", flagPort))
		return ln, flagPort, err
	}
	if envPort := strings.TrimSpace(os.Getenv("PORT")); envPort != "" {
		p, err := strconv.Atoi(envPort)
		if err != nil || p <= 0 || p > 65535 {
			return nil, 0, fmt.Errorf("invalid PORT env %q", envPort)
		}
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		return ln, p, err
	}
	var lastErr error
	for p := defaultPortStart; p <= defaultPortEnd; p++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			return ln, p, nil
		}
		lastErr = err
	}
	return nil, 0, fmt.Errorf("no open port in %d..%d: %w", defaultPortStart, defaultPortEnd, lastErr)
}

func resolveWorkDir(explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		if abs, err := filepath.Abs(explicit); err == nil {
			return abs
		}
	}
	cwd, _ := os.Getwd()
	exe, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exe)
		lowExe := strings.ToLower(exeDir)
		lowTmp := strings.ToLower(os.TempDir())
		if !strings.Contains(lowExe, "go-build") && !strings.HasPrefix(lowExe, lowTmp) {
			return exeDir
		}
	}
	return cwd
}
