package ffmpeg

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"
)

// ProgressSnapshot captures parsed FFmpeg -progress metrics.
type ProgressSnapshot struct {
	Progress      float64
	FPS           float64
	Speed         string
	TimeProcessed string
	ETA           string
}

// FormatClock converts seconds into HH:MM:SS.
func FormatClock(secs float64) string {
	if secs < 0 || math.IsNaN(secs) || math.IsInf(secs, 0) {
		secs = 0
	}
	total := int(math.Floor(secs))
	h := total / 3600
	m := (total % 3600) / 60
	s := total % 60
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

// FormatETA formats remaining seconds into a human-friendly string.
func FormatETA(remainingSecs float64) string {
	if remainingSecs <= 0 || math.IsNaN(remainingSecs) || math.IsInf(remainingSecs, 0) {
		return "0s"
	}
	secs := int(math.Ceil(remainingSecs))
	if secs < 60 {
		return fmt.Sprintf("%ds", secs)
	}
	if secs < 3600 {
		return fmt.Sprintf("%dm %ds", secs/60, secs%60)
	}
	return fmt.Sprintf("%dh %dm %ds", secs/3600, (secs%3600)/60, secs%60)
}

// ParseProgressStream parses FFmpeg -progress pipe:1 output and emits snapshots.
func ParseProgressStream(r io.Reader, totalTargetSecs, offsetSecs float64, startedAt time.Time, onUpdate func(ProgressSnapshot)) {
	scanner := bufio.NewScanner(r)
	cur := ProgressSnapshot{
		Speed:         "0x",
		TimeProcessed: FormatClock(offsetSecs),
		ETA:           "Calculating...",
	}
	var currentOutSecs float64

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)

		switch k {
		case "fps":
			if f, err := strconv.ParseFloat(v, 64); err == nil && f >= 0 {
				cur.FPS = math.Round(f*10) / 10
			}
		case "speed":
			if v != "" && v != "N/A" {
				cur.Speed = v
			}
		case "out_time_us":
			us, err := strconv.ParseInt(v, 10, 64)
			if err == nil {
				if us < 0 {
					us = 0
				}
				currentOutSecs = float64(us) / 1_000_000.0
				processed := offsetSecs + currentOutSecs
				cur.TimeProcessed = FormatClock(processed)
				if totalTargetSecs > 0 {
					pct := (processed / totalTargetSecs) * 100.0
					if pct < 0 {
						pct = 0
					}
					if pct > 99.9 {
						pct = 99.9
					}
					cur.Progress = math.Round(pct*10) / 10
					cur.ETA = estimateETA(totalTargetSecs, processed, cur.Speed, startedAt)
				}
			}
		case "progress":
			if onUpdate != nil {
				onUpdate(cur)
			}
		}
	}
}

func estimateETA(totalTargetSecs, processedSecs float64, speedStr string, startedAt time.Time) string {
	remaining := math.Max(0, totalTargetSecs-processedSecs)
	spd := parseSpeedMultiplier(speedStr)
	if spd > 0 {
		return FormatETA(remaining / spd)
	}
	elapsed := time.Since(startedAt).Seconds()
	if processedSecs > 0.05 && elapsed > 0.2 {
		empirical := processedSecs / elapsed
		if empirical > 0 {
			return FormatETA(remaining / empirical)
		}
	}
	return "Calculating..."
}

func parseSpeedMultiplier(raw string) float64 {
	s := strings.TrimSpace(strings.TrimSuffix(strings.ToLower(raw), "x"))
	if s == "" || s == "n/a" {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(v) || math.IsInf(v, 0) || v <= 0 {
		return 0
	}
	return v
}
