package mathutil

import (
	"math"
	"testing"
)

func almostEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

func TestComputeKeepIntervals(t *testing.T) {
	kept, err := ComputeKeepIntervals(10.0, nil)
	if err != nil || len(kept) != 1 || kept[0].Start != 0 || kept[0].End != 10.0 {
		t.Fatalf("expected full interval, got %+v, err=%v", kept, err)
	}

	// Overlapping, out-of-order, adjacent (<=0.001s), and out-of-bounds excludes.
	excludes := []Interval{
		{Start: 5.0, End: 7.0},
		{Start: -2.0, End: 1.0},
		{Start: 1.0005, End: 2.0},
		{Start: 6.5, End: 8.0},
		{Start: 9.97, End: 12.0},
	}
	kept, err = ComputeKeepIntervals(10.0, excludes)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	// Excluded merged: [0, 2.0] and [5.0, 8.0] and [9.97, 10.0] -> remaining [2.0, 5.0] and [8.0, 9.97]
	if len(kept) != 2 {
		t.Fatalf("expected 2 keep intervals, got %+v", kept)
	}
	if !almostEqual(kept[0].Start, 2.0, 1e-6) || !almostEqual(kept[0].End, 5.0, 1e-6) {
		t.Fatalf("unexpected kept[0]: %+v", kept[0])
	}
	if !almostEqual(kept[1].Start, 8.0, 1e-6) || !almostEqual(kept[1].End, 9.97, 1e-6) {
		t.Fatalf("unexpected kept[1]: %+v", kept[1])
	}

	// Tiny keep sliver (< 0.05s) dropped; full exclusion rejected.
	_, err = ComputeKeepIntervals(1.0, []Interval{{Start: 0.0, End: 0.96}})
	if err == nil {
		t.Fatalf("expected error when remaining kept duration < 0.05s")
	}
}

func TestIsTrimEffective(t *testing.T) {
	if IsTrimEffective([]Interval{{Start: 0, End: 10}}, 10) {
		t.Fatalf("full clip should not be effective trim")
	}
	if !IsTrimEffective([]Interval{{Start: 0.5, End: 10}}, 10) {
		t.Fatalf("trimmed start should be effective")
	}
	if !IsTrimEffective([]Interval{{Start: 0, End: 4}, {Start: 6, End: 10}}, 10) {
		t.Fatalf("multi-segment should be effective")
	}
}

func TestBuildAtempoChain(t *testing.T) {
	cases := []struct {
		speed   float64
		want    string
		wantErr bool
	}{
		{0.24, "", true},
		{8.01, "", true},
		{math.NaN(), "", true},
		{1.0, "", false},
		{0.25, "atempo=0.5,atempo=0.5", false},
		{0.3, "atempo=0.5,atempo=0.6", false},
		{0.5, "atempo=0.5", false},
		{1.5, "atempo=1.5", false},
		{2.0, "atempo=2.0", false},
		{4.0, "atempo=2.0,atempo=2.0", false},
		{6.0, "atempo=2.0,atempo=2.0,atempo=1.5", false},
		{8.0, "atempo=2.0,atempo=2.0,atempo=2.0", false},
	}
	for _, tc := range cases {
		got, err := BuildAtempoChain(tc.speed)
		if tc.wantErr {
			if err == nil {
				t.Errorf("speed %v: expected error, got %q", tc.speed, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("speed %v: unexpected error %v", tc.speed, err)
			continue
		}
		if got != tc.want {
			t.Errorf("speed %v: got %q, want %q", tc.speed, got, tc.want)
		}
	}
}

func TestNormalizeSplitPoints(t *testing.T) {
	segs, err := NormalizeSplitPoints([]float64{5.0, 0.0, 2.0, 2.02, 10.0, 9.98}, 10.0)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(segs) != 3 {
		t.Fatalf("expected 3 segments, got %+v", segs)
	}
	if !almostEqual(segs[0].End, 2.0, 1e-6) || !almostEqual(segs[1].End, 5.0, 1e-6) || !almostEqual(segs[2].End, 10.0, 1e-6) {
		t.Fatalf("unexpected segments: %+v", segs)
	}

	if _, err := NormalizeSplitPoints([]float64{0, 10.0}, 10.0); err == nil {
		t.Fatalf("expected error for boundary-only points")
	}
}

func TestUniformSplitBounds(t *testing.T) {
	// 3.01s split by 1.5s -> trailing 0.01s (<0.05s) must merge into segment 2 instead of creating 0ms sliver.
	segs, err := UniformSplitBounds(3.01, 1.5, 0)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if len(segs) != 2 {
		t.Fatalf("expected 2 segments (trailing sliver merged), got %+v", segs)
	}
	if !almostEqual(segs[1].End, 3.01, 1e-6) {
		t.Fatalf("expected final end 3.01, got %+v", segs[1])
	}

	// Equal parts mode.
	partSegs, err := UniformSplitBounds(9.0, 0, 3)
	if err != nil || len(partSegs) != 3 {
		t.Fatalf("expected 3 parts, got %+v, err=%v", partSegs, err)
	}
}
