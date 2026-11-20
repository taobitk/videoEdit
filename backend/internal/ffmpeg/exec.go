package ffmpeg

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"velocityengine/internal/syswin"
)

type ffmpegExecError struct {
	err    error
	stderr string
}

func (e *ffmpegExecError) Error() string {
	return fmt.Sprintf("%v: %s", e.err, summarizeStderr(e.stderr))
}

func summarizeStderr(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > 4 {
		lines = lines[len(lines)-4:]
	}
	return strings.Join(lines, " | ")
}

// RunFFmpegCommand executes FFmpeg, streaming progress and stderr logs and handling cancellation.
func RunFFmpegCommand(ctx context.Context, ffmpegPath string, args []string, totalTargetSecs, offsetSecs float64, hooks JobHooks) error {
	cmd := exec.Command(ffmpegPath, args...)
	cmd.SysProcAttr = syswin.HiddenProcAttr()

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	startedAt := time.Now()
	if err := cmd.Start(); err != nil {
		return err
	}
	pid := cmd.Process.Pid
	if hooks.SetPID != nil {
		hooks.SetPID(pid)
	}
	defer func() {
		if hooks.SetPID != nil {
			hooks.SetPID(0)
		}
	}()

	doneCh := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = syswin.KillProcessTree(pid)
			_ = cmd.Process.Kill()
		case <-doneCh:
		}
	}()

	var stderrBuf strings.Builder
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		sc := bufio.NewScanner(stderr)
		sc.Buffer(make([]byte, 4096), 256*1024)
		for sc.Scan() {
			line := strings.TrimSpace(sc.Text())
			if line != "" {
				stderrBuf.WriteString(line)
				stderrBuf.WriteByte('\n')
				if hooks.Log != nil {
					hooks.Log(line)
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		ParseProgressStream(stdout, totalTargetSecs, offsetSecs, startedAt, hooks.UpdateProgress)
	}()

	wg.Wait()
	waitErr := cmd.Wait()
	close(doneCh)

	if ctx.Err() != nil {
		return errors.New("render cancelled by user")
	}
	if waitErr != nil {
		return &ffmpegExecError{err: waitErr, stderr: stderrBuf.String()}
	}
	return nil
}
