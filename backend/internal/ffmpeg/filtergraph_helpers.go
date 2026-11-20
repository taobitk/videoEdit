package ffmpeg

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"velocityengine/internal/mathutil"
	"velocityengine/internal/model"
)

const (
	regularFontPath = `C:\Windows\Fonts\arial.ttf`
	boldFontPath    = `C:\Windows\Fonts\arialbd.ttf`
)

// EscapeFilterPath escapes a filesystem path for unquoted FFmpeg filtergraph options.
func EscapeFilterPath(rawPath string) string {
	p := strings.ReplaceAll(rawPath, `\`, `/`)
	var b strings.Builder
	b.Grow(len(p) + 16)
	for _, r := range p {
		switch r {
		case '\\':
			b.WriteString(`\\\\`)
		case ':':
			b.WriteString(`\\:`)
		case '\'':
			b.WriteString(`\\\'`)
		case '[':
			b.WriteString(`\[`)
		case ']':
			b.WriteString(`\]`)
		case ',':
			b.WriteString(`\,`)
		case ';':
			b.WriteString(`\;`)
		case ' ':
			b.WriteString(`\ `)
		case '=':
			b.WriteString(`\=`)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ColorFilterExpr returns the FFmpeg video filter expression matching frontend SVG feColorMatrix.
func ColorFilterExpr(name string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "none":
		return "", nil
	case "grayscale":
		return "hue=s=0", nil
	case "sepia":
		return "colorchannelmixer=.393:.769:.189:0:.349:.686:.168:0:.272:.534:.131", nil
	case "warm":
		return "colorchannelmixer=rr=1.12:gg=1.02:bb=0.86", nil
	case "cool":
		return "colorchannelmixer=rr=0.86:gg=1.00:bb=1.14", nil
	case "vibrant":
		return "hue=s=1.4", nil
	default:
		return "", fmt.Errorf("unknown video_filter %q", name)
	}
}

func resolveFontPath(fontWeight string) string {
	if strings.EqualFold(strings.TrimSpace(fontWeight), "bold") {
		if _, err := os.Stat(boldFontPath); err == nil {
			return boldFontPath
		}
	}
	return regularFontPath
}

func writeTempTextFile(idx int, rawText string) (string, error) {
	normalized := strings.ReplaceAll(rawText, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	if strings.TrimSpace(normalized) == "" {
		return "", nil
	}
	f, err := os.CreateTemp("", fmt.Sprintf("ve_drawtext_%d_*.txt", idx))
	if err != nil {
		return "", fmt.Errorf("create temp text file: %w", err)
	}
	path := f.Name()
	if _, err := f.WriteString(normalized); err != nil {
		f.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write temp text file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func buildDrawtextStep(layer model.LayerItem, textFilePath string, dispHeight int, outDur float64) string {
	fontFile := EscapeFilterPath(resolveFontPath(layer.FontWeight))
	txtFile := EscapeFilterPath(textFilePath)
	fontSize := mathutil.ComputeFontSize(dispHeight, layer.FontSize, layer.FontSizeRatio)

	xRatio := mathutil.ClampFloat(mathutil.ResolveOptionalFloat(layer.XRatio, 0.5), 0, 1)
	yRatio := mathutil.ClampFloat(mathutil.ResolveOptionalFloat(layer.YRatio, 0.5), 0, 1)
	opacity := mathutil.ClampFloat(mathutil.ResolveOptionalFloat(layer.Opacity, 100), 0, 100) / 100.0
	borderW := mathutil.ClampFloat(mathutil.ResolveOptionalFloat(layer.BorderWidth, 2), 0, 50)
	startPct := mathutil.ResolveOptionalFloat(layer.StartPct, 0)
	endPct := mathutil.ResolveOptionalFloat(layer.EndPct, 100)

	fontHex := mathutil.NormalizeHexColor(layer.FontColor, "FFFFFF")
	borderHex := mathutil.NormalizeHexColor(layer.BorderColor, "000000")

	var opts []string
	opts = append(opts,
		"textfile="+txtFile,
		"fontfile="+fontFile,
		"expansion=none",
		fmt.Sprintf("fontsize=%d", fontSize),
		fmt.Sprintf("fontcolor=0x%s@%.3f", fontHex, opacity),
		fmt.Sprintf("borderw=%.1f", borderW),
		fmt.Sprintf("bordercolor=0x%s@%.3f", borderHex, opacity),
	)
	if strings.Contains(strings.ReplaceAll(layer.Text, "\r\n", "\n"), "\n") {
		opts = append(opts, "text_align=M+C")
	}
	if layer.BgEnabled {
		bgHex := mathutil.NormalizeHexColor(layer.BgColor, "000000")
		bgOp := mathutil.ClampFloat(mathutil.ResolveOptionalFloat(layer.BgOpacity, 60), 0, 100) / 100.0
		bgPad := mathutil.ClampFloat(mathutil.ResolveOptionalFloat(layer.BgPadding, 6), 0, 100)
		opts = append(opts,
			"box=1",
			fmt.Sprintf("boxcolor=0x%s@%.3f", bgHex, bgOp*opacity),
			fmt.Sprintf("boxborderw=%.1f", bgPad),
		)
	}
	opts = append(opts,
		fmt.Sprintf("x=w*%.4f-text_w/2", xRatio),
		fmt.Sprintf("y=h*%.4f-text_h/2", yRatio),
		mathutil.ComputeOverlayEnable(outDur, startPct, endPct),
	)
	return "drawtext=" + strings.Join(opts, ":")
}

func writeTempRasterPNG(idx int, dataURL string) (string, error) {
	raw := strings.TrimSpace(dataURL)
	if comma := strings.Index(raw, ","); comma >= 0 && strings.HasPrefix(strings.ToLower(raw[:comma]), "data:image/") {
		raw = raw[comma+1:]
	}
	pngBytes, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return "", fmt.Errorf("decode raster_data_url: %w", err)
	}
	if !bytes.HasPrefix(pngBytes, []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}) {
		return "", fmt.Errorf("raster_data_url is not a valid PNG payload")
	}
	f, err := os.CreateTemp("", fmt.Sprintf("ve_raster_%d_*.png", idx))
	if err != nil {
		return "", fmt.Errorf("create temp raster png: %w", err)
	}
	path := f.Name()
	if _, err := f.Write(pngBytes); err != nil {
		f.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("write temp raster png: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

func resolveOverlayImagePath(uploadsDir string, layer model.LayerItem) (string, error) {
	candidate := strings.TrimSpace(layer.Path)
	if candidate == "" {
		candidate = strings.TrimSpace(layer.Filename)
	}
	if candidate == "" {
		return "", fmt.Errorf("image overlay missing path/filename")
	}
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return filepath.Abs(candidate)
	}
	if uploadsDir != "" {
		joined := filepath.Join(uploadsDir, filepath.Base(candidate))
		if info, err := os.Stat(joined); err == nil && !info.IsDir() {
			return filepath.Abs(joined)
		}
	}
	return "", fmt.Errorf("overlay image not found: %s", candidate)
}

func imageInputArgs(imgPath string) []string {
	ext := strings.ToLower(filepath.Ext(imgPath))
	if ext == ".gif" {
		return []string{"-stream_loop", "-1", "-i", imgPath}
	}
	return []string{"-loop", "1", "-i", imgPath}
}

// FormatConcatListLine formats a single file entry for FFmpeg concat demuxer escaping single quotes.
func FormatConcatListLine(absPath string) string {
	slash := strings.ReplaceAll(absPath, `\`, `/`)
	escaped := strings.ReplaceAll(slash, `'`, `'\''`)
	return fmt.Sprintf("file '%s'", escaped)
}
