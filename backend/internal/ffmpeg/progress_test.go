package ffmpeg

import (
	"strings"
	"testing"
	"time"
)

func TestParseProgressStreamClampsAndEstimatesETA(t *testing.T) {
	feed := strings.Join([]string{
		"fps=120.5",
		"speed=2.0x",
		"out_time_us=-50000",
		"progress=continue",
		"fps=150.0",
		"speed=2.0x",
		"out_time_us=5000000",
		"progress=continue",
		"out_time_us=12000000",
		"progress=end",
	}, "\n")

	var updates []ProgressSnapshot
	ParseProgressStream(strings.NewReader(feed), 10.0, 0.0, time.Now().Add(-time.Second), func(s ProgressSnapshot) {
		updates = append(updates, s)
	})
	if len(updates) != 3 {
		t.Fatalf("expected 3 snapshots, got %d", len(updates))
	}
	// First negative out_time_us clamped to 0.
	if updates[0].Progress != 0 || updates[0].TimeProcessed != "00:00:00" {
		t.Fatalf("expected clamped negative time to 0, got %+v", updates[0])
	}
	// Second update at 5s / 10s = 50%, speed 2x -> remaining 5s / 2 = 2.5s -> 3s.
	if updates[1].Progress != 50.0 || updates[1].ETA != "3s" {
		t.Fatalf("unexpected mid progress snapshot: %+v", updates[1])
	}
	// Clamped to 99.9 until final job completion marks 100.
	if updates[2].Progress != 99.9 {
		t.Fatalf("expected capped 99.9 during run, got %+v", updates[2])
	}
}

func TestErrorPatternClassifiers(t *testing.T) {
	if !IsHardwareNVENCError("[h264_nvenc] Cannot load nvcuda.dll") {
		t.Fatalf("expected nvcuda error detected")
	}
	if !IsHardwareNVENCError("No capable devices found") {
		t.Fatalf("expected No capable devices detected")
	}
	if IsHardwareNVENCError("No such file or directory") || IsCUDADecodeError("No such file or directory") {
		t.Fatalf("missing file must not trigger GPU fallback classifiers")
	}
	if IsHardwareNVENCError("Invalid argument in drawtext") || IsCUDADecodeError("Invalid argument in drawtext") {
		t.Fatalf("syntax error must not trigger GPU fallback classifiers")
	}
}
