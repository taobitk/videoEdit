//go:build windows

package syswin

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	comdlg32             = syscall.NewLazyDLL("comdlg32.dll")
	procGetOpenFileNameW = comdlg32.NewProc("GetOpenFileNameW")
	procCommDlgExtError  = comdlg32.NewProc("CommDlgExtendedError")
	procGetForegroundWnd = user32.NewProc("GetForegroundWindow")
)

const (
	ofnPathMustExist = 0x00000800
	ofnFileMustExist = 0x00001000
	ofnNoChangeDir   = 0x00000008
	ofnExplorer      = 0x00080000
)

type openFileNameW struct {
	lStructSize       uint32
	hwndOwner         uintptr
	hInstance         uintptr
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        unsafe.Pointer
	dwReserved        uint32
	flagsEx           uint32
}

// SelectFileDialog opens a native Win32 OpenFileDialog directly via comdlg32.dll.
func SelectFileDialog(ctx context.Context, fileType string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	title, filterPairs := dialogTitleAndFilter(fileType)
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	filterBuf := encodeDoubleNullUTF16(filterPairs)
	fileBuf := make([]uint16, 32768)
	hwndOwner, _, _ := procGetForegroundWnd.Call()

	ofn := openFileNameW{
		lStructSize:  uint32(unsafe.Sizeof(openFileNameW{})),
		hwndOwner:    hwndOwner,
		lpstrFilter:  &filterBuf[0],
		nFilterIndex: 1,
		lpstrFile:    &fileBuf[0],
		nMaxFile:     uint32(len(fileBuf)),
		lpstrTitle:   titlePtr,
		flags:        ofnExplorer | ofnFileMustExist | ofnPathMustExist | ofnNoChangeDir,
	}

	ok, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
	if ok == 0 {
		extErr, _, _ := procCommDlgExtError.Call()
		if extErr == 0 {
			return "", nil // User cancelled.
		}
		return "", fmt.Errorf("file dialog failed (CommDlgExtendedError=0x%x)", extErr)
	}

	return syscall.UTF16ToString(fileBuf), nil
}

func dialogTitleAndFilter(fileType string) (string, []string) {
	switch strings.ToLower(strings.TrimSpace(fileType)) {
	case "image":
		return "Chọn Ảnh Overlay", []string{
			"Image Files (*.png;*.jpg;*.jpeg;*.webp;*.gif;*.bmp)", "*.png;*.jpg;*.jpeg;*.webp;*.gif;*.bmp",
			"All Files (*.*)", "*.*",
		}
	default:
		return "Chọn Video File", []string{
			"Video Files (*.mp4;*.mkv;*.mov;*.webm;*.avi;*.flv;*.ts;*.m4v)", "*.mp4;*.mkv;*.mov;*.webm;*.avi;*.flv;*.ts;*.m4v",
			"All Files (*.*)", "*.*",
		}
	}
}

func encodeDoubleNullUTF16(pairs []string) []uint16 {
	var out []uint16
	for _, s := range pairs {
		out = append(out, utf16.Encode([]rune(s))...)
		out = append(out, 0)
	}
	out = append(out, 0)
	return out
}
