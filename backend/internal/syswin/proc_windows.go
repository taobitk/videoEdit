//go:build windows

package syswin

import (
	"fmt"
	"os/exec"
	"strconv"
	"syscall"
)

const createNoWindow = 0x08000000

// HiddenProcAttr returns Windows process attributes that suppress console windows.
func HiddenProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNoWindow,
	}
}

// KillProcessTree terminates a Windows process and all of its children via taskkill.
func KillProcessTree(pid int) error {
	if pid <= 0 {
		return nil
	}
	cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
	cmd.SysProcAttr = HiddenProcAttr()
	return cmd.Run()
}

// OpenInExplorer launches Windows Explorer selecting the given absolute file path.
func OpenInExplorer(absPath string) error {
	exe, err := exec.LookPath("explorer.exe")
	if err != nil {
		exe = "explorer.exe"
	}
	cmd := exec.Command(exe)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: fmt.Sprintf(`explorer.exe /select,"%s"`, absPath),
	}
	return cmd.Start()
}

// OpenBrowser opens the given URL in the user's default Windows browser.
func OpenBrowser(targetURL string) error {
	cmd := exec.Command("rundll32.exe", "url.dll,FileProtocolHandler", targetURL)
	cmd.SysProcAttr = HiddenProcAttr()
	return cmd.Start()
}
