package ffmpeg

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"velocityengine/internal/model"
)

func f64Ptr(v float64) *float64 { return &v }

func TestEscapeFilterPath(t *testing.T) {
	in := `C:\Users\O'Neil [1]\logo,final;v=2.png`
	got := EscapeFilterPath(in)
	if strings.Contains(got, `\Users`) {
		t.Fatalf("expected forward slashes, got %q", got)
	}
	for _, must := range []string{`\\:`, `\\\'`, `\[1\]`, `\,`, `\;`, `\=`, `\ `} {
		if !strings.Contains(got, must) {
			t.Errorf("expected %q in escaped path %q", must, got)
		}
	}
}

func TestFormatConcatListLine(t *testing.T) {
	got := FormatConcatListLine(`D:\Clips\O'Neil's video.mp4`)
	want := `file 'D:/Clips/O'\''Neil'\''s video.mp4'`
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestColorFilterExpr(t *testing.T) {
	want := map[string]string{
		"none":      "",
		"grayscale": "hue=s=0",
		"sepia":     "colorchannelmixer=.393:.769:.189:0:.349:.686:.168:0:.272:.534:.131",
		"warm":      "colorchannelmixer=rr=1.12:gg=1.02:bb=0.86",
		"cool":      "colorchannelmixer=rr=0.86:gg=1.00:bb=1.14",
		"vibrant":   "hue=s=1.4",
	}
	for k, w := range want {
		got, err := ColorFilterExpr(k)
		if err != nil || got != w {
			t.Errorf("ColorFilterExpr(%q) = (%q, %v), want %q", k, got, err, w)
		}
	}
	if _, err := ColorFilterExpr("bogus"); err == nil {
		t.Fatalf("expected error for unknown filter")
	}
}

func TestBuildProcessFiltergraphZOrderAndZeros(t *testing.T) {
	tmpDir := t.TempDir()
	imgPath := filepath.Join(tmpDir, "logo.png")
	if err := os.WriteFile(imgPath, []byte("fake"), 0644); err != nil {
		t.Fatal(err)
	}

	probe := ProbeInfo{
		ProbeResponse: model.ProbeResponse{
			Width: 1080, Height: 1920, Duration: 10.0, HasAudio: true,
		},
	}

	req := model.ProcessRequest{
		Speed:       f64Ptr(1.5),
		VideoFilter: "cool",
		TrimEnabled: true,
		TrimSegments: []model.TrimSegment{
			{Start: 1.0, End: 3.0},
		},
		Layers: []model.LayerItem{
			{
				ID: "l0", Type: "image", Path: imgPath, WidthRatio: 0.25,
				XRatio: f64Ptr(0.5), YRatio: f64Ptr(0.2), Opacity: f64Ptr(0),
				StartPct: f64Ptr(0), EndPct: f64Ptr(100),
			},
			{
				ID: "l1", Type: "text", Text: "Line 1 100%\r\nLine 2",
				FontSizeRatio: 0.05, FontWeight: "bold", FontColor: "#ff00aa",
				BorderColor: "#000", BorderWidth: f64Ptr(0),
				BgEnabled: true, BgColor: "#112233", BgOpacity: f64Ptr(50), BgPadding: f64Ptr(0),
				Opacity: f64Ptr(80), StartPct: f64Ptr(10), EndPct: f64Ptr(50),
			},
		},
	}

	plan, err := BuildProcessFiltergraph(req, probe, tmpDir)
	if err != nil {
		t.Fatalf("BuildProcessFiltergraph failed: %v", err)
	}
	defer plan.Cleanup()

	if len(plan.TempFiles) != 2 || !strings.HasSuffix(plan.TempFiles[0], ".txt") || !strings.HasSuffix(plan.TempFiles[1], ".glsl") {
		t.Fatalf("expected 1 temp .txt and 1 temp .glsl file, got %v", plan.TempFiles)
	}
	txtBytes, err := os.ReadFile(plan.TempFiles[0])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(txtBytes), "\r") || string(txtBytes) != "Line 1 100%\nLine 2" {
		t.Fatalf("expected LF-only text, got %q", string(txtBytes))
	}

	fc := plan.FilterComplex
	imgIdx := strings.Index(fc, "overlay=")
	txtIdx := strings.Index(fc, "drawtext=")
	if imgIdx < 0 || txtIdx < 0 || imgIdx > txtIdx {
		t.Fatalf("expected Z-order image before drawtext, got %s", fc)
	}
	for _, must := range []string{
		"trim=start=0.000:end=1.000,setpts=PTS-STARTPTS",
		"setpts=(PTS-STARTPTS)/1.5",
		"colorchannelmixer=rr=0.86:gg=1.00:bb=1.14",
		"colorchannelmixer=aa=0.000",
		"expansion=none",
		"text_align=M+C",
		"fontsize=96",
		"borderw=0.0",
		"boxborderw=0.0",
		"boxcolor=0x112233@0.400",
		"enable='gte(t,0.000)'",
		"enable='between(t,0.533,2.667)'",
		"scale=trunc(iw/2)*2:trunc(ih/2)*2[v]",
		"atempo=1.5,asetpts=PTS-STARTPTS[a]",
	} {
		if !strings.Contains(fc, must) {
			t.Errorf("filter_complex missing %q:\n%s", must, fc)
		}
	}
	if !strings.Contains(strings.ToLower(fc), "arialbd.ttf") {
		t.Logf("note: arialbd.ttf fallback used on non-standard font env")
	}
}

func TestBuildProcessFiltergraphRasterDataURL(t *testing.T) {
	// 1x1 transparent PNG base64
	tinyPNG := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVR42mNgAAIAAAUAAen63NgAAAAASUVORK5CYII="
	probe := ProbeInfo{
		ProbeResponse: model.ProbeResponse{Width: 1280, Height: 720, Duration: 5.0},
	}
	req := model.ProcessRequest{
		Speed: f64Ptr(1.0),
		Layers: []model.LayerItem{
			{
				ID: "txt-raster", Type: "image", RasterDataURL: tinyPNG,
				WidthRatio: 0.4, XRatio: f64Ptr(0.5), YRatio: f64Ptr(0.5),
				Opacity: f64Ptr(100), StartPct: f64Ptr(0), EndPct: f64Ptr(100),
			},
		},
	}
	plan, err := BuildProcessFiltergraph(req, probe, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer plan.Cleanup()
	if len(plan.TempFiles) != 1 || !strings.HasSuffix(plan.TempFiles[0], ".png") {
		t.Fatalf("expected 1 temp png, got %v", plan.TempFiles)
	}
	if !strings.Contains(plan.FilterComplex, "scale=512:-1") || strings.Contains(plan.FilterComplex, "drawtext=") {
		t.Fatalf("expected raster overlay without drawtext, got %s", plan.FilterComplex)
	}
}

func TestZeroCopy9x16PaddingAndGPUColorPlan(t *testing.T) {
	tinyPNG := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAAC0lEQVR42mNgAAIAAAUAAen63NgAAAAASUVORK5CYII="
	probe := ProbeInfo{ProbeResponse: model.ProbeResponse{Width: 720, Height: 1280, Duration: 4.0}}
	req := model.ProcessRequest{
		Speed:       f64Ptr(1.0),
		TrimEnabled: true,
		TrimSegments: []model.TrimSegment{{Start: 1.0, End: 2.0}},
		Layers: []model.LayerItem{{
			ID: "ov9x16", Type: "image", RasterDataURL: tinyPNG,
			WidthRatio: 0.25, XRatio: f64Ptr(0.5), YRatio: f64Ptr(0.5),
			Opacity: f64Ptr(100), StartPct: f64Ptr(0), EndPct: f64Ptr(100),
		}},
	}
	plan, err := BuildProcessFiltergraph(req, probe, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer plan.Cleanup()
	if plan.ZeroCopyPadRight != 48 {
		t.Fatalf("expected ZeroCopyPadRight=48 for 720px width, got %d", plan.ZeroCopyPadRight)
	}
	for _, must := range []string{"pad_cuda=w=768:h=1280", "overlay_cuda=x=720*0.5000", "concat=n=2"} {
		if !strings.Contains(plan.ZeroCopyCUDAFilterComplex, must) {
			t.Fatalf("ZeroCopyCUDAFilterComplex missing %q:\n%s", must, plan.ZeroCopyCUDAFilterComplex)
		}
	}

	reqColor := model.ProcessRequest{Speed: f64Ptr(1.0), VideoFilter: "sepia"}
	colorPlan, err := BuildProcessFiltergraph(reqColor, probe, "")
	if err != nil {
		t.Fatalf("unexpected color error: %v", err)
	}
	if !strings.Contains(colorPlan.GPUColorFilterComplex, "libplacebo=custom_shader_path=") {
		t.Fatalf("expected libplacebo custom_shader_path, got %s", colorPlan.GPUColorFilterComplex)
	}
	if len(colorPlan.TempFiles) != 1 || !strings.HasSuffix(colorPlan.TempFiles[0], ".glsl") {
		t.Fatalf("expected 1 temp .glsl file, got %v", colorPlan.TempFiles)
	}
	shaderPath := colorPlan.TempFiles[0]
	colorPlan.Cleanup()
	if _, statErr := os.Stat(shaderPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected shader temp file %s to be cleaned up", shaderPath)
	}
}
