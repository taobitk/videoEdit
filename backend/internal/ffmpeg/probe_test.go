package ffmpeg

import (
	"testing"

	"velocityengine/internal/model"
)

func TestBuildProbeInfoBitrateExtraction(t *testing.T) {
	t.Run("prefers_video_stream_bitrate", func(t *testing.T) {
		parsed := ffprobeJSON{
			Streams: []ffprobeStream{{
				CodecType: "video", CodecName: "vp9",
				Width: 1080, Height: 1920,
				BitRate: "602066",
			}},
			Format: ffprobeFormat{
				Duration: "170.033333",
				BitRate:  "676060",
			},
		}
		info, err := buildProbeInfo(`C:\tmp\v.mp4`, 14369102, parsed)
		if err != nil {
			t.Fatal(err)
		}
		if info.BitrateKbps != 602 {
			t.Fatalf("expected 602 kbps from stream, got %d", info.BitrateKbps)
		}
	})

	t.Run("falls_back_to_format_bitrate", func(t *testing.T) {
		parsed := ffprobeJSON{
			Streams: []ffprobeStream{{
				CodecType: "video", CodecName: "vp9",
				Width: 1080, Height: 1920,
				BitRate: "N/A",
			}},
			Format: ffprobeFormat{
				Duration: "170.033333",
				BitRate:  "676060",
			},
		}
		info, err := buildProbeInfo(`C:\tmp\v.mp4`, 14369102, parsed)
		if err != nil {
			t.Fatal(err)
		}
		if info.BitrateKbps != 676 {
			t.Fatalf("expected 676 kbps from format, got %d", info.BitrateKbps)
		}
	})

	t.Run("falls_back_to_filesize_over_duration", func(t *testing.T) {
		parsed := ffprobeJSON{
			Streams: []ffprobeStream{{
				CodecType: "video", CodecName: "vp9",
				Width: 1080, Height: 1920,
			}},
			Format: ffprobeFormat{
				Duration: "170.033333",
			},
		}
		info, err := buildProbeInfo(`C:\tmp\v.mp4`, 14369102, parsed)
		if err != nil {
			t.Fatal(err)
		}
		if info.BitrateKbps != 676 {
			t.Fatalf("expected 676 kbps from fileSize/duration, got %d", info.BitrateKbps)
		}
	})
}

func TestNormalizeQualityMode(t *testing.T) {
	cases := map[string]string{
		"":         "auto_cap",
		"auto_cap": "auto_cap",
		"COMPACT":  "compact",
		" high ":   "high",
		"invalid":  "auto_cap",
	}
	for in, want := range cases {
		if got := model.NormalizeQualityMode(in); got != want {
			t.Errorf("NormalizeQualityMode(%q) = %q, want %q", in, got, want)
		}
	}
}
