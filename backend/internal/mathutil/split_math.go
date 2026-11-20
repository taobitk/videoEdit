package mathutil

import (
	"fmt"
	"math"
	"sort"
)

// NormalizeSplitPoints clamps, sorts, deduplicates split cut points and converts to intervals.
func NormalizeSplitPoints(points []float64, duration float64) ([]Interval, error) {
	if math.IsNaN(duration) || math.IsInf(duration, 0) || duration < 2*minSegmentSeconds {
		return nil, fmt.Errorf("video too short to split: %.3fs", duration)
	}
	var valid []float64
	for _, p := range points {
		if math.IsNaN(p) || math.IsInf(p, 0) {
			continue
		}
		if p <= minSegmentSeconds || p >= duration-minSegmentSeconds {
			continue
		}
		valid = append(valid, p)
	}
	sort.Float64s(valid)

	var dedup []float64
	for _, p := range valid {
		if len(dedup) == 0 || p-dedup[len(dedup)-1] >= minSegmentSeconds {
			dedup = append(dedup, p)
		}
	}
	if len(dedup) == 0 {
		return nil, fmt.Errorf("no valid split points inside (%.2fs, %.2fs)", minSegmentSeconds, duration-minSegmentSeconds)
	}

	bounds := make([]float64, 0, len(dedup)+2)
	bounds = append(bounds, 0)
	bounds = append(bounds, dedup...)
	bounds = append(bounds, duration)

	segs := make([]Interval, 0, len(bounds)-1)
	for i := 0; i < len(bounds)-1; i++ {
		if bounds[i+1]-bounds[i] >= minSegmentSeconds {
			segs = append(segs, Interval{Start: bounds[i], End: bounds[i+1]})
		}
	}
	if len(segs) < 2 {
		return nil, fmt.Errorf("split points yield fewer than 2 valid segments")
	}
	return segs, nil
}

// UniformSplitBounds splits duration into fixed-length segments or equal parts.
func UniformSplitBounds(duration, segmentDuration float64, parts int) ([]Interval, error) {
	if math.IsNaN(duration) || math.IsInf(duration, 0) || duration < minSegmentSeconds {
		return nil, fmt.Errorf("invalid duration for split: %.3fs", duration)
	}
	if parts >= 2 && segmentDuration <= 0 {
		segmentDuration = duration / float64(parts)
	}
	if math.IsNaN(segmentDuration) || math.IsInf(segmentDuration, 0) || segmentDuration < minSegmentSeconds {
		return nil, fmt.Errorf("segment_duration must be >= %.2fs", minSegmentSeconds)
	}
	if segmentDuration >= duration-minSegmentSeconds {
		return nil, fmt.Errorf("segment_duration %.2fs >= video duration %.2fs", segmentDuration, duration)
	}

	var segs []Interval
	cursor := 0.0
	for cursor < duration-minSegmentSeconds {
		next := cursor + segmentDuration
		if duration-next < minSegmentSeconds {
			next = duration
		}
		segs = append(segs, Interval{Start: cursor, End: next})
		cursor = next
	}
	if len(segs) < 2 {
		return nil, fmt.Errorf("uniform split yields fewer than 2 segments")
	}
	return segs, nil
}
