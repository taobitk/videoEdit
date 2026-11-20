package server

import (
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"velocityengine/internal/ffmpeg"
)

func TestE2EHybridCudaToSvtAv1(t *testing.T) {
	srv, ts, tmp := newRealE2EServer(t)
	ff := srv.cfg.Bins.FFmpeg
	src := filepath.Join(tmp, "Hybrid AV1 Tiếng Việt.mp4")
	if out, err := exec.Command(ff, "-y", "-f", "lavfi", "-i", "color=c=0x3366cc:s=320x180:r=30:d=1",
		"-f", "lavfi", "-i", "sine=frequency=440:duration=1",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", "-c:a", "aac", src).CombinedOutput(); err != nil {
		t.Fatalf("generate source failed: %v\n%s", err, out)
	}

	mustPostJSON(t, ts.URL+"/api/process", map[string]any{
		"source_type": "local", "local_path": src,
		"video_filter": "sepia", "use_gpu": true, "codec": "av1", "preset": "p1", "cq": 28, "speed": 1.0,
	})
	st := waitForJob(t, ts.URL, 30*time.Second)
	if st.Status != "completed" || !strings.Contains(st.EncoderDetail, "SVT-AV1") {
		t.Fatalf("expected Hybrid SVT-AV1 completion, got status=%s detail=%q logs=%v", st.Status, st.EncoderDetail, st.Logs)
	}
	if st.ActiveEncoder != "HYBRID" || st.FPS <= 0 {
		t.Fatalf("expected HYBRID active encoder and fps > 0, got enc=%q fps=%.1f", st.ActiveEncoder, st.FPS)
	}
	outPath := filepath.Join(srv.cfg.DownloadsDir, st.OutputFilename)
	outProbe, err := ffmpeg.ProbeFile(context.Background(), srv.cfg.Bins.FFprobe, outPath)
	if err != nil || outProbe.VideoCodec != "av1" || !outProbe.HasAudio {
		t.Fatalf("unexpected AV1 output probe: %+v err=%v", outProbe, err)
	}
	r, g, b := probeCenterRGB(t, ff, outPath, 0.2)
	if r <= g || g <= b || r <= 120 {
		t.Fatalf("expected sepia-filtered AV1 center pixel, got RGB=(%d,%d,%d)", r, g, b)
	}
}
