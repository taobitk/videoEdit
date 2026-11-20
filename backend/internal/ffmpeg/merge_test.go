package ffmpeg

import (
	"strings"
	"testing"

	"velocityengine/internal/model"
)

func TestAreClipsCopyCompatible(t *testing.T) {
	base := ProbeInfo{
		ProbeResponse: model.ProbeResponse{
			Width: 640, Height: 360, RawWidth: 640, RawHeight: 360,
			Rotation: 0, Duration: 3.0, FPS: 30, VideoCodec: "h264",
			HasAudio: true, AudioCodec: "aac",
		},
		PixFmt: "yuv420p", AudioSampleRate: 48000, AudioChannels: 2,
	}
	ok, reason := AreClipsCopyCompatible([]ProbeInfo{base, base})
	if !ok {
		t.Fatalf("expected compatible, got reason: %s", reason)
	}

	rotClip := base
	rotClip.Rotation = 90
	rotClip.Width, rotClip.Height = 360, 640
	if ok, _ := AreClipsCopyCompatible([]ProbeInfo{base, rotClip}); ok {
		t.Fatalf("expected rotation mismatch to reject stream copy")
	}

	noAudio := base
	noAudio.HasAudio = false
	if ok, _ := AreClipsCopyCompatible([]ProbeInfo{base, noAudio}); ok {
		t.Fatalf("expected audio presence mismatch to reject stream copy")
	}
}

func TestBuildMergeFiltergraphDynamicResolutionAndAnullsrc(t *testing.T) {
	clip1 := ProbeInfo{
		ProbeResponse: model.ProbeResponse{
			Width: 640, Height: 360, RawWidth: 640, RawHeight: 360,
			Duration: 3.0, FPS: 30, VideoCodec: "h264", HasAudio: true, AudioCodec: "aac",
		},
	}
	clip2 := ProbeInfo{
		ProbeResponse: model.ProbeResponse{
			Width: 360, Height: 640, RawWidth: 640, RawHeight: 360,
			Rotation: 90, Duration: 2.5, FPS: 29.97, VideoCodec: "vp9", HasAudio: false,
		},
	}

	fc, maps := BuildMergeFiltergraph([]ProbeInfo{clip1, clip2})
	if strings.Contains(fc, "1920") || strings.Contains(fc, "1080") {
		t.Fatalf("filtergraph must not hardcode 1920x1080: %s", fc)
	}
	for _, must := range []string{
		"scale=w=640:h=360:force_original_aspect_ratio=decrease,pad=640:360:(ow-iw)/2:(oh-ih)/2:black,setsar=1,fps=30,format=yuv420p,setpts=PTS-STARTPTS[v0]",
		"aresample=48000,aformat=sample_fmts=fltp:channel_layouts=stereo,asetpts=PTS-STARTPTS[a0]",
		"anullsrc=channel_layout=stereo:sample_rate=48000,atrim=duration=2.500,asetpts=PTS-STARTPTS[a1]",
		"[v0][a0][v1][a1]concat=n=2:v=1:a=1[v][a]",
	} {
		if !strings.Contains(fc, must) {
			t.Errorf("merge filtergraph missing %q:\n%s", must, fc)
		}
	}
	if len(maps) != 4 {
		t.Fatalf("expected 4 map args, got %v", maps)
	}

	zcFC, _ := BuildZeroCopyMergeFiltergraph([]ProbeInfo{clip1, clip2})
	if !strings.Contains(zcFC, "[1:v]transpose_cuda=dir=cclock,fps=30,scale_cuda=") {
		t.Fatalf("expected transpose_cuda in zero-copy merge filtergraph, got:\n%s", zcFC)
	}
}
