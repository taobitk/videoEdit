package mathutil

import (
	"fmt"
	"math"
	"strings"
)

// ClampFloat clamps v to [minVal, maxVal].
func ClampFloat(v, minVal, maxVal float64) float64 {
	if math.IsNaN(v) {
		return minVal
	}
	if v < minVal {
		return minVal
	}
	if v > maxVal {
		return maxVal
	}
	return v
}

// ResolveOptionalFloat returns *ptr when non-nil, otherwise defaultVal.
func ResolveOptionalFloat(ptr *float64, defaultVal float64) float64 {
	if ptr == nil {
		return defaultVal
	}
	return *ptr
}

// MakeEven rounds n up to the next even integer, enforcing a minimum of minVal.
func MakeEven(n, minVal int) int {
	if n < minVal {
		n = minVal
	}
	if n%2 != 0 {
		n++
	}
	return n
}

// ComputeOverlayImageWidth computes an even pixel width >= 10 from display width and ratio.
func ComputeOverlayImageWidth(dispWidth int, widthRatio float64) int {
	if dispWidth <= 0 {
		dispWidth = 1920
	}
	ratio := ClampFloat(widthRatio, 0.01, 2.0)
	w := int(math.Round(float64(dispWidth) * ratio))
	return MakeEven(w, 10)
}

// ComputeFontSize computes font pixel size using font_size_ratio when > 0, else font_size.
func ComputeFontSize(dispHeight int, fontSize, fontSizeRatio float64) int {
	if fontSizeRatio > 0 && dispHeight > 0 {
		px := int(math.Round(float64(dispHeight) * fontSizeRatio))
		if px < 8 {
			return 8
		}
		return px
	}
	px := int(math.Round(fontSize))
	if px <= 0 {
		return 32
	}
	return px
}

// NormalizeHexColor normalizes #RGB or #RRGGBB into uppercase "RRGGBB" (without prefix).
func NormalizeHexColor(raw, fallback string) string {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "#")
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	if len(s) == 3 {
		s = string([]byte{s[0], s[0], s[1], s[1], s[2], s[2]})
	}
	if len(s) != 6 {
		return strings.ToUpper(strings.TrimPrefix(fallback, "#"))
	}
	for i := 0; i < 6; i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return strings.ToUpper(strings.TrimPrefix(fallback, "#"))
		}
	}
	return strings.ToUpper(s)
}

// ComputeOverlayEnable builds FFmpeg drawtext/overlay enable expression on output duration.
func ComputeOverlayEnable(outputDuration, startPct, endPct float64) string {
	sPct := ClampFloat(startPct, 0, 100)
	ePct := ClampFloat(endPct, 0, 100)
	if ePct < sPct {
		ePct = sPct
	}
	t0 := outputDuration * (sPct / 100.0)
	if t0 < 0 {
		t0 = 0
	}
	if ePct >= 99.95 {
		return fmt.Sprintf("enable='gte(t,%.3f)'", t0)
	}
	t1 := outputDuration * (ePct / 100.0)
	if t1 < t0 {
		t1 = t0
	}
	return fmt.Sprintf("enable='between(t,%.3f,%.3f)'", t0, t1)
}
