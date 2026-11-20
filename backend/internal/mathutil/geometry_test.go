package mathutil

import "testing"

func TestComputeOverlayImageWidth(t *testing.T) {
	if got := ComputeOverlayImageWidth(1080, 0.2); got != 216 {
		t.Fatalf("expected 216, got %d", got)
	}
	// Odd pixel rounded up to even.
	if got := ComputeOverlayImageWidth(361, 0.25); got%2 != 0 || got < 90 {
		t.Fatalf("expected even >=90, got %d", got)
	}
	// Minimum clamped to 10.
	if got := ComputeOverlayImageWidth(100, 0.02); got != 10 {
		t.Fatalf("expected 10, got %d", got)
	}
}

func TestComputeFontSize(t *testing.T) {
	if got := ComputeFontSize(1920, 32, 0.05); got != 96 {
		t.Fatalf("expected 96 from ratio, got %d", got)
	}
	if got := ComputeFontSize(1920, 44, 0); got != 44 {
		t.Fatalf("expected 44 fallback, got %d", got)
	}
	if got := ComputeFontSize(1920, 0, 0); got != 32 {
		t.Fatalf("expected default 32, got %d", got)
	}
}

func TestNormalizeHexColor(t *testing.T) {
	cases := []struct {
		in, fb, want string
	}{
		{"#fff", "000000", "FFFFFF"},
		{"#1a2b3c", "000000", "1A2B3C"},
		{"0xabcdef", "000000", "ABCDEF"},
		{"invalid", "#FFFFFF", "FFFFFF"},
	}
	for _, tc := range cases {
		if got := NormalizeHexColor(tc.in, tc.fb); got != tc.want {
			t.Errorf("NormalizeHexColor(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestComputeOverlayEnable(t *testing.T) {
	if got := ComputeOverlayEnable(10.0, 0, 100); got != "enable='gte(t,0.000)'" {
		t.Fatalf("expected gte at 100%%, got %q", got)
	}
	if got := ComputeOverlayEnable(10.0, 25, 99.95); got != "enable='gte(t,2.500)'" {
		t.Fatalf("expected gte at 99.95%%, got %q", got)
	}
	if got := ComputeOverlayEnable(10.0, 20, 50); got != "enable='between(t,2.000,5.000)'" {
		t.Fatalf("expected between, got %q", got)
	}
	// Explicit 0% endPct must remain 0.000, not fallback to 100%.
	if got := ComputeOverlayEnable(10.0, 0, 0); got != "enable='between(t,0.000,0.000)'" {
		t.Fatalf("expected zero endPct preserved, got %q", got)
	}
}

func TestResolveOptionalFloatZero(t *testing.T) {
	zero := 0.0
	if got := ResolveOptionalFloat(&zero, 100.0); got != 0.0 {
		t.Fatalf("expected explicit 0.0 preserved, got %v", got)
	}
	if got := ResolveOptionalFloat(nil, 100.0); got != 100.0 {
		t.Fatalf("expected nil to default to 100.0, got %v", got)
	}
}
