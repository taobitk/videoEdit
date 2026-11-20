package ffmpeg

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_RUNNER_HELPER") != "1" {
		return
	}
	scenario := os.Getenv("RUNNER_SCENARIO")
	args := strings.Join(os.Args[1:], " ")
	hasCUDA := strings.Contains(args, "-hwaccel cuda")
	hasNVENC := strings.Contains(args, "h264_nvenc")

	switch scenario {
	case "tier2_succeeds":
		if hasCUDA {
			fmt.Fprintln(os.Stderr, "Cannot load nvcuda or cuvid hwaccel decode error")
			os.Exit(1)
		}
		if !hasNVENC {
			fmt.Fprintln(os.Stderr, "expected nvenc on tier 2")
			os.Exit(2)
		}
		fmt.Println("progress=end")
		os.Exit(0)

	case "tier3_cpu_fallback":
		if hasNVENC {
			fmt.Fprintln(os.Stderr, "[h264_nvenc] No capable devices found")
			os.Exit(1)
		}
		fmt.Println("progress=end")
		os.Exit(0)

	case "syntax_no_fallback":
		fmt.Fprintln(os.Stderr, "Invalid argument in filtergraph drawtext")
		os.Exit(1)

	case "sleep_cancel":
		time.Sleep(10 * time.Second)
		os.Exit(0)
	}
	os.Exit(0)
}

func writeHelperBatch(t *testing.T) string {
	t.Helper()
	batPath := filepath.Join(t.TempDir(), "ffmpeg_mock.cmd")
	content := fmt.Sprintf("@echo off\r\n\"%s\" -test.run=^TestHelperProcess$ -- %%*\r\n", os.Args[0])
	if err := os.WriteFile(batPath, []byte(content), 0755); err != nil {
		t.Fatal(err)
	}
	return batPath
}

func runHelperScenario(t *testing.T, scenario string) (error, []string, []string) {
	t.Helper()
	t.Setenv("GO_WANT_RUNNER_HELPER", "1")
	t.Setenv("RUNNER_SCENARIO", scenario)
	batPath := writeHelperBatch(t)

	var encoders, warnings []string
	outPath := filepath.Join(t.TempDir(), "out.mp4")
	hooks := JobHooks{
		SetEncoder: func(active, detail string) { encoders = append(encoders, active+":"+detail) },
		Warn:       func(msg string) { warnings = append(warnings, msg) },
	}
	err := RunWith3TierFallback(
		context.Background(),
		batPath,
		[]VideoInput{{Path: "in.mp4"}},
		nil,
		nil,
		outPath,
		5.0,
		0,
		EncoderConfig{UseGPU: true, Codec: "h264", Preset: "p4"},
		hooks,
	)
	return err, encoders, warnings
}

func TestRunWith3TierFallback(t *testing.T) {
	// Tier 1 CUDA decode error -> Tier 2 CPU decode + NVENC succeeds with no CPU warning.
	err, encs, warns := runHelperScenario(t, "tier2_succeeds")
	if err != nil {
		t.Fatalf("expected tier 2 success, got err=%v", err)
	}
	if len(encs) != 2 || len(warns) != 0 {
		t.Fatalf("expected 2 attempts and 0 warnings, got encs=%v warns=%v", encs, warns)
	}

	// Tier 1 + Tier 2 NVENC fail -> Tier 3 CPU fallback + Vietnamese warning.
	err, encs, warns = runHelperScenario(t, "tier3_cpu_fallback")
	if err != nil {
		t.Fatalf("expected tier 3 CPU fallback success, got err=%v", err)
	}
	if len(encs) < 2 || len(warns) != 1 || !strings.Contains(warns[0], "NVIDIA") {
		t.Fatalf("expected fallback warning, got encs=%v warns=%v", encs, warns)
	}

	// Non-GPU syntax error must NOT trigger CPU fallback or false NVIDIA warning.
	err, encs, warns = runHelperScenario(t, "syntax_no_fallback")
	if err == nil {
		t.Fatalf("expected syntax error to fail")
	}
	if len(encs) != 1 || len(warns) != 0 {
		t.Fatalf("syntax error must not fallback or warn, got encs=%v warns=%v", encs, warns)
	}
}

func TestRunFFmpegCommandCancel(t *testing.T) {
	t.Setenv("GO_WANT_RUNNER_HELPER", "1")
	t.Setenv("RUNNER_SCENARIO", "sleep_cancel")
	batPath := writeHelperBatch(t)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	err := RunFFmpegCommand(ctx, batPath, []string{"-y"}, 5.0, 0, JobHooks{})
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancel error, got %v", err)
	}
}

func TestEncoderArgsForTierQualityModes(t *testing.T) {
	cases := []struct {
		name        string
		cfg         EncoderConfig
		tier        execTier
		mustContain []string
		mustExclude []string
	}{
		{
			name: "default_zero_cq_uses_20",
			cfg: EncoderConfig{
				UseGPU: true, Codec: "hevc", Preset: "p4", CQ: 0,
			},
			tier:        tierCUDAAndNVENC,
			mustContain: []string{"-c:v hevc_nvenc", "-rc vbr -b:v 0 -cq 20 -spatial_aq 1", "-tag:v hvc1"},
			mustExclude: []string{"-maxrate", "-bufsize"},
		},
		{
			name: "custom_cq_24_hevc_nvenc",
			cfg: EncoderConfig{
				UseGPU: true, Codec: "hevc", Preset: "p7", CQ: 24,
			},
			tier:        tierCUDAAndNVENC,
			mustContain: []string{"-c:v hevc_nvenc", "-preset p7", "-rc vbr -b:v 0 -cq 24 -spatial_aq 1"},
			mustExclude: []string{"-maxrate", "-bufsize"},
		},
		{
			name: "custom_cq_28_clamped_from_35_cpu",
			cfg: EncoderConfig{
				UseGPU: false, Codec: "hevc", Preset: "p7", CQ: 35,
			},
			tier:        tierCPUEncode,
			mustContain: []string{"-c:v libx265", "-crf 28", "-tag:v hvc1"},
			mustExclude: []string{"-maxrate", "-bufsize"},
		},
		{
			name: "clamped_low_cq_12_uses_20",
			cfg: EncoderConfig{
				UseGPU: false, Codec: "h264", Preset: "p4", CQ: 12,
			},
			tier:        tierCPUEncode,
			mustContain: []string{"-c:v libx264", "-crf 20"},
			mustExclude: []string{"-maxrate", "-bufsize"},
		},
		{
			name: "av1_libsvtav1_hybrid_maps_preset_and_crf",
			cfg: EncoderConfig{
				UseGPU: true, Codec: "av1", Preset: "8", CQ: 30,
			},
			tier:        tierCPUEncode,
			mustContain: []string{"-c:v libsvtav1 -preset 8 -crf 30 -pix_fmt yuv420p"},
			mustExclude: []string{"-maxrate", "-bufsize"},
		},
		{
			name: "av1_clamped_high_crf_45_to_38",
			cfg: EncoderConfig{
				UseGPU: true, Codec: "av1", Preset: "p1", CQ: 45,
			},
			tier:        tierCPUEncode,
			mustContain: []string{"-c:v libsvtav1 -preset 10 -crf 38"},
			mustExclude: []string{"-maxrate", "-bufsize"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			args, _, _ := encoderArgsForTier(tc.cfg, tc.tier)
			joined := strings.Join(args, " ")
			for _, want := range tc.mustContain {
				if !strings.Contains(joined, want) {
					t.Errorf("expected args %q to contain %q", joined, want)
				}
			}
			for _, forbid := range tc.mustExclude {
				if strings.Contains(joined, forbid) {
					t.Errorf("expected args %q NOT to contain %q", joined, forbid)
				}
			}
		})
	}
}
