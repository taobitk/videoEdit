//go:build !windows

package syswin

import (
	"fmt"
	"os/exec"
	"syscall"
)

// HiddenProcAttr returns nil on non-Windows platforms.
func HiddenProcAttr() *syscall.SysProcAttr {
	return nil
}

// KillProcessTree kills the process on non-Windows platforms.
func KillProcessTree(pid int) error {
	if pid <= 0 {
		return nil
	}
	return syscall.Kill(pid, syscall.SIGKILL)
}

// OpenInExplorer is unsupported outside Windows.
func OpenInExplorer(absPath string) error {
	return fmt.Errorf("open in explorer unsupported on this OS: %s", absPath)
}

// OpenBrowser opens a URL using xdg-open on non-Windows systems.
func OpenBrowser(targetURL string) error {
	return exec.Command("xdg-open", targetURL).Start()
}
