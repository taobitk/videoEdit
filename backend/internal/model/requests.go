package model

import (
	"fmt"
	"path/filepath"
	"strings"
)

// TrimSegment defines an excluded time range in seconds.
type TrimSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
}

// LayerItem represents a unified Z-ordered text or image overlay layer.
type LayerItem struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"`
	Text          string   `json:"text"`
	FontSize      float64  `json:"font_size"`
	FontSizeRatio float64  `json:"font_size_ratio"`
	FontWeight    string   `json:"font_weight"`
	FontColor     string   `json:"font_color"`
	BorderColor   string   `json:"border_color"`
	BorderWidth   *float64 `json:"border_width"`
	BgEnabled     bool     `json:"bg_enabled"`
	BgColor       string   `json:"bg_color"`
	BgOpacity     *float64 `json:"bg_opacity"`
	BgPadding     *float64 `json:"bg_padding"`
	Filename      string   `json:"filename"`
	Path          string   `json:"path"`
	RasterDataURL string   `json:"raster_data_url"`
	WidthRatio    float64  `json:"width_ratio"`
	XRatio        *float64 `json:"x_ratio"`
	YRatio        *float64 `json:"y_ratio"`
	Opacity       *float64 `json:"opacity"`
	StartPct      *float64 `json:"start_pct"`
	EndPct        *float64 `json:"end_pct"`
}

// SelectFileRequest is the payload for POST /api/select_file.
type SelectFileRequest struct {
	FileType string `json:"file_type"`
}

// ProbeRequest is the payload for POST /api/probe.
type ProbeRequest struct {
	Path string `json:"path"`
}

// ShowExplorerRequest is the payload for POST /api/show_explorer.
type ShowExplorerRequest struct {
	Filename string `json:"filename"`
}

// ProcessRequest is the payload for POST /api/process.
type ProcessRequest struct {
	SourceType    string        `json:"source_type"`
	Filename      string        `json:"filename"`
	LocalPath     string        `json:"local_path"`
	Path          string        `json:"path"`
	Speed         *float64      `json:"speed"`
	UseGPU        *bool         `json:"use_gpu"`
	Codec         string        `json:"codec"`
	Preset        string        `json:"preset"`
	QualityMode   string        `json:"quality_mode"`
	CQ            int           `json:"cq"`
	VideoFilter   string        `json:"video_filter"`
	TrimEnabled   bool          `json:"trim_enabled"`
	TrimSegments  []TrimSegment `json:"trim_segments"`
	Layers        []LayerItem   `json:"layers"`
	TextOverlay   *LayerItem    `json:"text_overlay"`
	TextOverlays  []LayerItem   `json:"text_overlays"`
	ImageOverlays []LayerItem   `json:"image_overlays"`
}

// SplitRequest is the payload for POST /api/split.
type SplitRequest struct {
	SourceType      string    `json:"source_type"`
	Filename        string    `json:"filename"`
	LocalPath       string    `json:"local_path"`
	Path            string    `json:"path"`
	Mode            string    `json:"mode"`
	SplitPoints     []float64 `json:"split_points"`
	SegmentDuration float64   `json:"segment_duration"`
	Parts           int       `json:"parts"`
	Accurate        bool      `json:"accurate"`
	UseGPU          *bool     `json:"use_gpu"`
	Codec           string    `json:"codec"`
	Preset          string    `json:"preset"`
	QualityMode     string    `json:"quality_mode"`
	CQ              int       `json:"cq"`
}

// MergeFileEntry identifies a single input clip for POST /api/merge.
type MergeFileEntry struct {
	Type     string `json:"type"`
	Filename string `json:"filename"`
	Path     string `json:"path"`
}

// MergeRequest is the payload for POST /api/merge.
type MergeRequest struct {
	Files       []MergeFileEntry `json:"files"`
	UseGPU      *bool            `json:"use_gpu"`
	Codec       string           `json:"codec"`
	Preset      string           `json:"preset"`
	QualityMode string           `json:"quality_mode"`
	CQ          int              `json:"cq"`
}

// ResolveLayers returns Z-ordered layers, falling back to legacy overlay slices.
func (r *ProcessRequest) ResolveLayers() []LayerItem {
	if len(r.Layers) > 0 {
		out := make([]LayerItem, len(r.Layers))
		copy(out, r.Layers)
		return out
	}
	var out []LayerItem
	texts := r.TextOverlays
	if len(texts) == 0 && r.TextOverlay != nil {
		texts = []LayerItem{*r.TextOverlay}
	}
	for _, t := range texts {
		t.Type = "text"
		out = append(out, t)
	}
	for _, im := range r.ImageOverlays {
		im.Type = "image"
		out = append(out, im)
	}
	return out
}

// NormalizeCodec validates and normalizes codec to "h264" or "hevc".
func NormalizeCodec(raw string) (string, error) {
	c := strings.ToLower(strings.TrimSpace(raw))
	switch c {
	case "", "h264", "avc", "x264":
		return "h264", nil
	case "hevc", "h265", "x265":
		return "hevc", nil
	case "av1", "svtav1", "libsvtav1":
		return "av1", nil
	default:
		return "", fmt.Errorf("unsupported codec %q (expected h264, hevc or av1)", raw)
	}
}

// NormalizeQualityMode normalizes quality_mode to "auto_cap", "compact", or "high".
func NormalizeQualityMode(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "compact":
		return "compact"
	case "high":
		return "high"
	default:
		return "auto_cap"
	}
}

// NormalizeCQ clamps CQ to the safe [20, 28] range for H.264/HEVC, defaulting to 20.
func NormalizeCQ(cq int) int {
	return NormalizeCQForCodec("h264", cq)
}

// NormalizeCQForCodec normalizes quality target per codec (AV1 CRF 24..38 def 30; NVENC CQ 20..28 def 20).
func NormalizeCQForCodec(codec string, cq int) int {
	if strings.EqualFold(strings.TrimSpace(codec), "av1") || strings.EqualFold(strings.TrimSpace(codec), "svtav1") {
		if cq <= 0 {
			return 30
		}
		if cq < 24 {
			return 24
		}
		if cq > 38 {
			return 38
		}
		return cq
	}
	if cq <= 0 {
		return 20
	}
	if cq < 20 {
		return 20
	}
	if cq > 28 {
		return 28
	}
	return cq
}

// ResolveInputPath resolves either a local file path or an uploaded filename inside uploadsDir.
func ResolveInputPath(uploadsDir, sourceType, filename, localPath, pathAlias string) (string, error) {
	st := strings.ToLower(strings.TrimSpace(sourceType))
	if st == "upload" && strings.TrimSpace(filename) != "" {
		base := filepath.Base(strings.TrimSpace(filename))
		if base == "" || base == "." || base == ".." {
			return "", fmt.Errorf("invalid uploaded filename")
		}
		return filepath.Abs(filepath.Join(uploadsDir, base))
	}
	p := strings.TrimSpace(localPath)
	if p == "" {
		p = strings.TrimSpace(pathAlias)
	}
	if p == "" {
		return "", fmt.Errorf("missing input file path")
	}
	return filepath.Abs(p)
}
