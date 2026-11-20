package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"velocityengine/internal/ffmpeg"
	"velocityengine/internal/model"
)

func findRepoRoot(t *testing.T) string {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	dir := wd
	for range 6 {
		if _, err := os.Stat(filepath.Join(dir, "velocity-cuda.exe")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Skip("velocity-cuda.exe not found; skipping real-GPU E2E test")
	return ""
}

func newRealE2EServer(t *testing.T) (*Server, *httptest.Server, string) {
	t.Helper()
	root := findRepoRoot(t)
	origWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWd) })

	bins := ffmpeg.ResolveBinaries()
	if bins.FFmpeg == "" || bins.FFprobe == "" {
		t.Skip("ffmpeg/ffprobe not available")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	caps := ffmpeg.DetectCapabilities(ctx, bins)
	if !caps.HasNvidiaGPU {
		t.Skip("NVIDIA GPU not detected; skipping CUDA E2E test")
	}

	tmp := t.TempDir()
	srv, err := New(context.Background(), Config{
		WorkDir:      tmp,
		StaticFS:     fstest.MapFS{"index.html": {Data: []byte("<html></html>")}},
		Bins:         bins,
		Capabilities: caps,
	})
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)
	return srv, ts, tmp
}

func mustPostJSON(t *testing.T, url string, payload any) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("POST %s failed: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s returned %d", url, resp.StatusCode)
	}
}

func waitForJob(t *testing.T, baseURL string, timeout time.Duration) model.EngineStatus {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(baseURL + "/api/status")
		if err != nil {
			t.Fatal(err)
		}
		var st model.EngineStatus
		err = json.NewDecoder(resp.Body).Decode(&st)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if st.Status == "completed" || st.Status == "failed" {
			return st
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatalf("job timed out after %v", timeout)
	return model.EngineStatus{}
}

func probeCenterRGB(t *testing.T, ffmpegBin, videoPath string, atSec float64) (int, int, int) {
	t.Helper()
	args := []string{"-v", "error", "-ss", fmt.Sprintf("%.2f", atSec), "-i", videoPath,
		"-vframes", "1", "-vf", "format=rgb24,crop=1:1:160:90", "-f", "rawvideo", "-pix_fmt", "rgb24", "-"}
	out, err := exec.Command(ffmpegBin, args...).Output()
	if err != nil || len(out) < 3 {
		t.Fatalf("probeCenterRGB(%s @ %.2fs) failed: %v (len=%d)", videoPath, atSec, err, len(out))
	}
	return int(out[0]), int(out[1]), int(out[2])
}

func TestE2ECudaVietnameseAV1OverlayTrimSpeedAndTelemetry(t *testing.T) {
	srv, ts, tmp := newRealE2EServer(t)
	ff := srv.cfg.Bins.FFmpeg

	vnAV1 := filepath.Join(tmp, "CHO BẠN THÂN VAY 180 TRIỆU — Test Tiếng Việt (AV1).mp4")
	ovPNG := filepath.Join(tmp, "ov.png")
	if out, err := exec.Command(ff, "-y", "-f", "lavfi", "-i", "color=c=0x3366cc:s=320x180:r=30:d=3",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=3",
		"-c:v", "libsvtav1", "-preset", "12", "-pix_fmt", "yuv420p", "-c:a", "aac", vnAV1).CombinedOutput(); err != nil {
		t.Fatalf("generate AV1 failed: %v\n%s", err, out)
	}
	if out, err := exec.Command(ff, "-y", "-f", "lavfi", "-i", "color=c=red@0.9:s=100x50:d=1",
		"-frames:v", "1", "-pix_fmt", "rgba", ovPNG).CombinedOutput(); err != nil {
		t.Fatalf("generate overlay PNG failed: %v\n%s", err, out)
	}
	pngBytes, err := os.ReadFile(ovPNG)
	if err != nil {
		t.Fatal(err)
	}
	dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngBytes)

	sseCtx, sseCancel := context.WithCancel(context.Background())
	defer sseCancel()
	sseCh := make(chan model.EngineStatus, 32)
	go func() {
		req, _ := http.NewRequestWithContext(sseCtx, http.MethodGet, ts.URL+"/api/events", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return
		}
		defer resp.Body.Close()
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			line := sc.Text()
			if strings.HasPrefix(line, "data: {") {
				var st model.EngineStatus
				if json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &st) == nil {
					sseCh <- st
				}
			}
		}
	}()

	mustPostJSON(t, ts.URL+"/api/process", map[string]any{
		"source_type":   "local",
		"local_path":    vnAV1,
		"video_filter":  "cool",
		"use_gpu":       true,
		"codec":         "hevc",
		"preset":        "p1",
		"quality_mode":  "compact",
		"speed":         1.2,
		"trim_enabled":  true,
		"trim_segments": []map[string]float64{{"start": 1.0, "end": 2.0}},
		"layers": []map[string]any{{
			"id": "l1", "type": "text", "raster_data_url": dataURL,
			"x_ratio": 0.5, "y_ratio": 0.5, "width_ratio": 0.3125,
			"opacity": 100, "start_pct": 0, "end_pct": 50,
		}},
	})

	st := waitForJob(t, ts.URL, 45*time.Second)
	sseCancel()
	if st.Status != "completed" {
		t.Fatalf("expected completed, got %+v", st)
	}
	if !strings.Contains(st.EncoderDetail, "CUDA NVRTC Zero-Copy+NVENC") {
		t.Fatalf("expected CUDA NVRTC Zero-Copy+NVENC, got %q (logs=%v)", st.EncoderDetail, st.Logs)
	}
	for _, line := range st.Logs {
		if strings.Contains(line, "WARNING: CUDA NVRTC runner failed") {
			t.Fatalf("unexpected fallback warning in logs: %s", line)
		}
	}
	if st.FPS <= 0 || st.Speed == "" || st.Speed == "0x" || st.Speed == "0.00x" {
		t.Fatalf("expected live fps > 0 & speed != 0x, got fps=%.1f speed=%q", st.FPS, st.Speed)
	}
	if len(sseCh) == 0 {
		t.Fatalf("expected SSE telemetry events")
	}

	outPath := filepath.Join(srv.cfg.DownloadsDir, st.OutputFilename)
	outProbe, err := ffmpeg.ProbeFile(context.Background(), srv.cfg.Bins.FFprobe, outPath)
	if err != nil {
		t.Fatal(err)
	}
	if !outProbe.HasAudio || math.Abs(outProbe.Duration-(2.0/1.2)) > 0.35 {
		t.Fatalf("unexpected output probe: audio=%v dur=%.3f (want ~1.67s)", outProbe.HasAudio, outProbe.Duration)
	}
	r0, g0, _ := probeCenterRGB(t, ff, outPath, 0.2)
	if r0 <= 180 || g0 >= 50 {
		t.Fatalf("expected active red overlay at t=0.2s, got R=%d G=%d", r0, g0)
	}
	r1, _, b1 := probeCenterRGB(t, ff, outPath, 1.2)
	if b1 <= 210 || r1 >= 50 {
		t.Fatalf("expected cool filtered bg after overlay ends at t=1.2s, got R=%d B=%d", r1, b1)
	}
}

func TestE2ECudaAllColorFiltersPixelAccuracy(t *testing.T) {
	srv, ts, tmp := newRealE2EServer(t)
	ff := srv.cfg.Bins.FFmpeg
	src := filepath.Join(tmp, "Test Màu Tiếng Việt.mp4")
	if out, err := exec.Command(ff, "-y", "-f", "lavfi", "-i", "color=c=0x3366cc:s=320x180:r=30:d=1",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", src).CombinedOutput(); err != nil {
		t.Fatalf("generate H264 failed: %v\n%s", err, out)
	}

	cases := []struct {
		name  string
		check func(r, g, b int) bool
	}{
		{"grayscale", func(r, g, b int) bool { return math.Abs(float64(r-g)) <= 3 && math.Abs(float64(g-b)) <= 3 }},
		{"sepia", func(r, g, b int) bool { return r > g && g > b && r > 120 }},
		{"warm", func(r, g, b int) bool { return r > 51 && b < 190 }},
		{"cool", func(r, g, b int) bool { return b > 215 && r < 50 }},
		{"vibrant", func(r, g, b int) bool { return (b - r) > 190 }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mustPostJSON(t, ts.URL+"/api/process", map[string]any{
				"source_type": "local", "local_path": src,
				"video_filter": tc.name, "use_gpu": true, "codec": "h264", "speed": 1.0,
			})
			st := waitForJob(t, ts.URL, 20*time.Second)
			if st.Status != "completed" || !strings.Contains(st.EncoderDetail, "CUDA NVRTC Zero-Copy+NVENC") {
				t.Fatalf("%s failed: status=%s enc=%q", tc.name, st.Status, st.EncoderDetail)
			}
			if st.FPS <= 0 {
				t.Fatalf("%s expected fps > 0, got %.1f", tc.name, st.FPS)
			}
			outPath := filepath.Join(srv.cfg.DownloadsDir, st.OutputFilename)
			r, g, b := probeCenterRGB(t, ff, outPath, 0.2)
			if !tc.check(r, g, b) {
				t.Fatalf("%s pixel mismatch: RGB=(%d,%d,%d)", tc.name, r, g, b)
			}
		})
	}
}

func TestE2ESplitAndMergeVietnameseFiles(t *testing.T) {
	srv, ts, tmp := newRealE2EServer(t)
	ff := srv.cfg.Bins.FFmpeg
	src := filepath.Join(tmp, "Cắt Ghép Tiếng Việt.mp4")
	if out, err := exec.Command(ff, "-y", "-f", "lavfi", "-i", "testsrc2=size=320x180:rate=30:d=2",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=2",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", src).CombinedOutput(); err != nil {
		t.Fatalf("generate split/merge input failed: %v\n%s", err, out)
	}

	mustPostJSON(t, ts.URL+"/api/split", map[string]any{
		"source_type": "local", "local_path": src,
		"mode": "markers", "split_points": []float64{1.0}, "accurate": true,
		"use_gpu": true, "codec": "h264",
	})
	st := waitForJob(t, ts.URL, 30*time.Second)
	if st.Status != "completed" || len(st.OutputFiles) != 2 {
		t.Fatalf("split failed: %+v", st)
	}

	var files []map[string]string
	for _, f := range st.OutputFiles {
		files = append(files, map[string]string{"type": "local", "path": filepath.Join(srv.cfg.DownloadsDir, f.Filename)})
	}
	mustPostJSON(t, ts.URL+"/api/merge", map[string]any{
		"files": files, "use_gpu": true, "codec": "h264",
	})
	stM := waitForJob(t, ts.URL, 30*time.Second)
	if stM.Status != "completed" || stM.OutputFilename == "" {
		t.Fatalf("merge failed: %+v", stM)
	}
	p, err := ffmpeg.ProbeFile(context.Background(), srv.cfg.Bins.FFprobe, filepath.Join(srv.cfg.DownloadsDir, stM.OutputFilename))
	if err != nil || math.Abs(p.Duration-2.0) > 0.3 {
		t.Fatalf("unexpected merged duration: dur=%.3f err=%v", p.Duration, err)
	}
}

func TestE2ECudaVisualQualityAndBitrateFloor(t *testing.T) {
	srv, ts, tmp := newRealE2EServer(t)
	ff := srv.cfg.Bins.FFmpeg
	src := filepath.Join(tmp, "Chất Lượng Hình Ảnh 720p.mp4")
	if out, err := exec.Command(ff, "-y", "-f", "lavfi", "-i", "testsrc2=size=1280x720:rate=30:d=2",
		"-c:v", "libx264", "-crf", "18", "-pix_fmt", "yuv420p", src).CombinedOutput(); err != nil {
		t.Fatalf("generate 720p testsrc2 failed: %v\n%s", err, out)
	}
	const transparent1x1 = "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	mustPostJSON(t, ts.URL+"/api/process", map[string]any{
		"source_type": "local", "local_path": src, "video_filter": "none",
		"use_gpu": true, "codec": "hevc", "preset": "p4", "quality_mode": "high", "speed": 1.0,
		"layers": []map[string]any{{"id": "q", "type": "text", "raster_data_url": transparent1x1, "x_ratio": 0.5, "y_ratio": 0.5, "width_ratio": 0.05, "opacity": 0, "start_pct": 0, "end_pct": 100}},
	})
	st := waitForJob(t, ts.URL, 30*time.Second)
	if st.Status != "completed" || !strings.Contains(st.EncoderDetail, "CUDA NVRTC Zero-Copy+NVENC") {
		t.Fatalf("unexpected status/enc: %s %q", st.Status, st.EncoderDetail)
	}
	outPath := filepath.Join(srv.cfg.DownloadsDir, st.OutputFilename)
	outProbe, err := ffmpeg.ProbeFile(context.Background(), srv.cfg.Bins.FFprobe, outPath)
	if err != nil || outProbe.BitrateKbps < 500 {
		t.Fatalf("expected high quality bitrate >= 500 kbps (no 200k choke), got %d kbps (err=%v)", outProbe.BitrateKbps, err)
	}
	rawSSIM, err := exec.Command(ff, "-i", src, "-i", outPath, "-lavfi", "ssim", "-f", "null", "-").CombinedOutput()
	if err != nil {
		t.Fatalf("ssim command failed: %v\n%s", err, rawSSIM)
	}
	m := regexp.MustCompile(`All:([0-9.]+)`).FindSubmatch(rawSSIM)
	if len(m) < 2 {
		t.Fatalf("failed to parse SSIM from: %s", rawSSIM)
	}
	ssimVal, _ := strconv.ParseFloat(string(m[1]), 64)
	if ssimVal < 0.97 {
		t.Fatalf("expected high visual quality SSIM >= 0.97, got %.4f (bitrate=%d kbps)", ssimVal, outProbe.BitrateKbps)
	}
}
