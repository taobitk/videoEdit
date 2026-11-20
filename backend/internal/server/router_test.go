package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"velocityengine/internal/model"
)

func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	tmp := t.TempDir()
	staticFS := fstest.MapFS{
		"index.html": {Data: []byte("<!doctype html><html><body>OK</body></html>")},
	}
	srv, err := New(context.Background(), Config{
		WorkDir:  tmp,
		StaticFS: staticFS,
		Capabilities: model.SystemCapabilities{
			FFmpegAvailable:  true,
			FFprobeAvailable: true,
			HasNvidiaGPU:     true,
			GPUName:          "Test RTX 3060",
			NvencH264:        true,
			NvencHEVC:        true,
		},
		SelectFileFn: func(_ context.Context, _ string) (string, error) {
			return "", nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	return srv, tmp
}

func TestCapabilitiesAndStatusAndSPA(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// SPA root
	resp, err := http.Get(ts.URL + "/")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("GET / failed: %v %v", resp, err)
	}
	resp.Body.Close()

	// Unknown API path returns 404 JSON, not SPA HTML.
	resp, err = http.Get(ts.URL + "/api/nonexistent")
	if err != nil || resp.StatusCode != 404 {
		t.Fatalf("expected 404 for unknown api route, got %v %v", resp, err)
	}
	resp.Body.Close()

	// Capabilities
	resp, err = http.Get(ts.URL + "/api/system/capabilities")
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("capabilities failed: %v", err)
	}
	var caps model.SystemCapabilities
	if err := json.NewDecoder(resp.Body).Decode(&caps); err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if !caps.HasNvidiaGPU || caps.GPUName != "Test RTX 3060" {
		t.Fatalf("unexpected caps: %+v", caps)
	}

	// Status slices must be [] not null.
	resp, err = http.Get(ts.URL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if strings.Contains(string(raw), `"output_files":null`) || strings.Contains(string(raw), `"warnings":null`) || strings.Contains(string(raw), `"logs":null`) {
		t.Fatalf("status JSON must not contain null arrays: %s", string(raw))
	}
}

func TestSSEInitialEvent(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/events", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/event-stream") {
		t.Fatalf("expected text/event-stream, got %q", ct)
	}
	sc := bufio.NewScanner(resp.Body)
	if !sc.Scan() {
		t.Fatalf("expected SSE line, err=%v", sc.Err())
	}
	if !strings.HasPrefix(sc.Text(), "data: {") {
		t.Fatalf("expected data JSON frame, got %q", sc.Text())
	}
}

func TestProbeAndValidationErrors(t *testing.T) {
	srv, tmp := newTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Probe directory -> 400
	body, _ := json.Marshal(map[string]string{"path": tmp})
	resp, err := http.Post(ts.URL+"/api/probe", "application/json", bytes.NewReader(body))
	if err != nil || resp.StatusCode != 400 {
		t.Fatalf("expected 400 on dir probe, got %v %v", resp.StatusCode, err)
	}
	resp.Body.Close()

	// Probe missing file -> 404
	body, _ = json.Marshal(map[string]string{"path": filepath.Join(tmp, "nope.mp4")})
	resp, err = http.Post(ts.URL+"/api/probe", "application/json", bytes.NewReader(body))
	if err != nil || resp.StatusCode != 404 {
		t.Fatalf("expected 404 on missing probe, got %v %v", resp.StatusCode, err)
	}
	resp.Body.Close()

	// Process invalid speed -> 400
	resp, err = http.Post(ts.URL+"/api/process", "application/json", strings.NewReader(`{"local_path":"a.mp4","speed":0.1}`))
	if err != nil || resp.StatusCode != 400 {
		t.Fatalf("expected 400 on speed 0.1, got %v %v", resp.StatusCode, err)
	}
	resp.Body.Close()

	// Merge with < 2 files -> 400
	resp, err = http.Post(ts.URL+"/api/merge", "application/json", strings.NewReader(`{"files":[{"path":"a.mp4"}]}`))
	if err != nil || resp.StatusCode != 400 {
		t.Fatalf("expected 400 on merge <2 files, got %v %v", resp.StatusCode, err)
	}
	resp.Body.Close()
}

func TestPreviewRangeAndDownloadRFC5987(t *testing.T) {
	srv, tmp := newTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	svgFile := filepath.Join(tmp, "icon.svg")
	if err := os.WriteFile(svgFile, []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), 0644); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get(ts.URL + "/api/preview?path=" + svgFile)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("preview svg failed: %v", err)
	}
	resp.Body.Close()
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "image/svg+xml") {
		t.Fatalf("expected image/svg+xml, got %q", ct)
	}

	// Range request on downloads + RFC 5987 attachment
	dlName := "test video.mp4"
	dlPath := filepath.Join(srv.cfg.DownloadsDir, dlName)
	if err := os.WriteFile(dlPath, []byte("0123456789ABCDEF"), 0644); err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/downloads/test%20video.mp4?download=1", nil)
	req.Header.Set("Range", "bytes=0-3")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != 206 || string(data) != "0123" {
		t.Fatalf("expected 206 '0123', got %d %q", resp.StatusCode, string(data))
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "filename*=UTF-8''test%20video.mp4") {
		t.Fatalf("expected RFC 5987 Content-Disposition, got %q", cd)
	}
}

func TestConflict409AndReset(t *testing.T) {
	srv, _ := newTestServer(t)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	_, ok := srv.State().TryStartJob(context.Background(), "process", "running", "out.mp4")
	if !ok {
		t.Fatal("failed to enter processing state")
	}

	for _, ep := range []string{"/api/process", "/api/split", "/api/merge", "/api/reset"} {
		resp, err := http.Post(ts.URL+ep, "application/json", strings.NewReader(`{}`))
		if err != nil || resp.StatusCode != 409 {
			t.Fatalf("expected 409 on %s while processing, got %v %v", ep, resp.StatusCode, err)
		}
		resp.Body.Close()
	}

	srv.State().FinishJob(nil, nil, "out.mp4", "/downloads/out.mp4", "Done")
	resp, err := http.Post(ts.URL+"/api/reset", "application/json", nil)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("expected 200 on reset after done, got %v %v", resp.StatusCode, err)
	}
	resp.Body.Close()
	if snap := srv.State().Snapshot(); snap.Status != "idle" || snap.OutputURL != "" || len(snap.OutputFiles) != 0 {
		t.Fatalf("reset did not clear state cleanly: %+v", snap)
	}
}
