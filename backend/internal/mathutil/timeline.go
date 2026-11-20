package mathutil

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	minSegmentSeconds = 0.05
	mergeEpsilon      = 0.001
	MinSpeed          = 0.25
	MaxSpeed          = 8.0
)

// Interval represents a half-open [Start, End] media segment in seconds.
type Interval struct {
	Start float64
	End   float64
}

// Duration returns the positive length of the interval.
func (iv Interval) Duration() float64 {
	if iv.End <= iv.Start {
		return 0
	}
	return iv.End - iv.Start
}

// ComputeKeepIntervals inverts excluded segments over [0, duration] into keep segments.
func ComputeKeepIntervals(duration float64, excludes []Interval) ([]Interval, error) {
	if math.IsNaN(duration) || math.IsInf(duration, 0) || duration < minSegmentSeconds {
		return nil, fmt.Errorf("invalid media duration: %.3f", duration)
	}
	if len(excludes) == 0 {
		return []Interval{{Start: 0, End: duration}}, nil
	}

	clamped := make([]Interval, 0, len(excludes))
	for _, ex := range excludes {
		if math.IsNaN(ex.Start) || math.IsNaN(ex.End) {
			continue
		}
		s := math.Max(0, math.Min(duration, ex.Start))
		e := math.Max(0, math.Min(duration, ex.End))
		if e > s {
			clamped = append(clamped, Interval{Start: s, End: e})
		}
	}
	if len(clamped) == 0 {
		return []Interval{{Start: 0, End: duration}}, nil
	}

	sort.Slice(clamped, func(i, j int) bool {
		if clamped[i].Start == clamped[j].Start {
			return clamped[i].End < clamped[j].End
		}
		return clamped[i].Start < clamped[j].Start
	})

	merged := make([]Interval, 0, len(clamped))
	cur := clamped[0]
	for _, next := range clamped[1:] {
		if next.Start <= cur.End+mergeEpsilon {
			if next.End > cur.End {
				cur.End = next.End
			}
		} else {
			merged = append(merged, cur)
			cur = next
		}
	}
	merged = append(merged, cur)

	var kept []Interval
	cursor := 0.0
	for _, ex := range merged {
		if ex.Start-cursor >= minSegmentSeconds {
			kept = append(kept, Interval{Start: cursor, End: ex.Start})
		}
		if ex.End > cursor {
			cursor = ex.End
		}
	}
	if duration-cursor >= minSegmentSeconds {
		kept = append(kept, Interval{Start: cursor, End: duration})
	}

	totalKept := TotalDuration(kept)
	if len(kept) == 0 || totalKept < minSegmentSeconds {
		return nil, fmt.Errorf("all segments were trimmed out (kept %.3fs < %.2fs)", totalKept, minSegmentSeconds)
	}
	return kept, nil
}

// TotalDuration returns the sum of durations across intervals.
func TotalDuration(intervals []Interval) float64 {
	var total float64
	for _, iv := range intervals {
		total += iv.Duration()
	}
	return total
}

// IsTrimEffective reports whether kept segments differ meaningfully from the full clip.
func IsTrimEffective(kept []Interval, originalDuration float64) bool {
	if len(kept) > 1 {
		return true
	}
	if len(kept) == 1 {
		return kept[0].Start > mergeEpsilon || kept[0].End < originalDuration-mergeEpsilon
	}
	return false
}

// ValidateSpeed ensures speed lies within [0.25, 8.0].
func ValidateSpeed(speed float64) error {
	if math.IsNaN(speed) || math.IsInf(speed, 0) || speed < MinSpeed || speed > MaxSpeed {
		return fmt.Errorf("speed %.4g out of allowed range [%.2f, %.2f]", speed, MinSpeed, MaxSpeed)
	}
	return nil
}

// BuildAtempoChain builds FFmpeg chained atempo filters for speed in [0.25, 8.0].
func BuildAtempoChain(speed float64) (string, error) {
	if err := ValidateSpeed(speed); err != nil {
		return "", err
	}
	if math.Abs(speed-1.0) <= 1e-4 {
		return "", nil
	}
	var parts []string
	rem := speed
	for rem > 2.0+1e-9 {
		parts = append(parts, "atempo=2.0")
		rem /= 2.0
	}
	for rem < 0.5-1e-9 {
		parts = append(parts, "atempo=0.5")
		rem /= 0.5
	}
	if math.Abs(rem-1.0) > 1e-4 {
		parts = append(parts, "atempo="+formatAtempoFactor(rem))
	}
	return strings.Join(parts, ","), nil
}

func formatAtempoFactor(v float64) string {
	s := strconv.FormatFloat(v, 'f', 4, 64)
	s = strings.TrimRight(s, "0")
	if strings.HasSuffix(s, ".") {
		s += "0"
	}
	return s
}
