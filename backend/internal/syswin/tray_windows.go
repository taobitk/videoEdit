//go:build windows

package syswin

import (
	"context"
	"runtime"
	"syscall"
	"unsafe"
)

var (
	user32               = syscall.NewLazyDLL("user32.dll")
	shell32              = syscall.NewLazyDLL("shell32.dll")
	gdi32                = syscall.NewLazyDLL("gdi32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	procGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	procRegisterClassExW = user32.NewProc("RegisterClassExW")
	procCreateWindowExW  = user32.NewProc("CreateWindowExW")
	procDestroyWindow    = user32.NewProc("DestroyWindow")
	procDefWindowProcW   = user32.NewProc("DefWindowProcW")
	procGetMessageW      = user32.NewProc("GetMessageW")
	procTranslateMessage = user32.NewProc("TranslateMessage")
	procDispatchMessageW = user32.NewProc("DispatchMessageW")
	procPostQuitMessage  = user32.NewProc("PostQuitMessage")
	procPostMessageW     = user32.NewProc("PostMessageW")
	procSetForegroundWnd = user32.NewProc("SetForegroundWindow")
	procGetCursorPos     = user32.NewProc("GetCursorPos")
	procCreatePopupMenu  = user32.NewProc("CreatePopupMenu")
	procAppendMenuW      = user32.NewProc("AppendMenuW")
	procTrackPopupMenu   = user32.NewProc("TrackPopupMenu")
	procDestroyMenu      = user32.NewProc("DestroyMenu")
	procCreateIconInd    = user32.NewProc("CreateIconIndirect")
	procDestroyIcon      = user32.NewProc("DestroyIcon")
	procCreateBitmap     = gdi32.NewProc("CreateBitmap")
	procDeleteObject     = gdi32.NewProc("DeleteObject")
	procShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
)

const (
	wmDestroy       = 0x0002
	wmClose         = 0x0010
	wmNull          = 0x0000
	wmCommand       = 0x0111
	wmLButtonDblClk = 0x0203
	wmRButtonUp     = 0x0205
	wmContextMenu   = 0x007B
	wmTrayCallback  = 0x8001 // WM_APP + 1

	nimAdd    = 0x00000000
	nimDelete = 0x00000002
	nifMsg    = 0x00000001
	nifIcon   = 0x00000002
	nifTip    = 0x00000004

	mfString      = 0x00000000
	mfSeparator   = 0x00000800
	tpmLeftAlign  = 0x0000
	tpmBottomAlign = 0x0020
	tpmRightButton = 0x0002

	cmdOpenBrowser = 1001
	cmdExitApp     = 1002
)

type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type point struct{ X, Y int32 }

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

type notifyIconDataW struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
}

// RunTray displays a Windows system tray icon with Open and Exit actions until closed or ctx cancels.
func RunTray(ctx context.Context, tooltip string, onOpen func(), onQuit func()) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	hInst, _, _ := procGetModuleHandleW.Call(0)
	clsName, _ := syscall.UTF16PtrFromString("VelocityEngineTrayWnd")
	var hwnd uintptr
	var hIcon uintptr

	wndProc := syscall.NewCallback(func(h uintptr, m uint32, wParam, lParam uintptr) uintptr {
		switch m {
		case wmTrayCallback:
			switch uint32(lParam & 0xFFFF) {
			case wmLButtonDblClk:
				if onOpen != nil {
					go onOpen()
				}
			case wmRButtonUp, wmContextMenu:
				showTrayMenu(h, onOpen)
			}
			return 0
		case wmCommand:
			switch uint32(wParam & 0xFFFF) {
			case cmdOpenBrowser:
				if onOpen != nil {
					go onOpen()
				}
			case cmdExitApp:
				_, _, _ = procPostMessageW.Call(h, wmClose, 0, 0)
			}
			return 0
		case wmClose:
			_, _, _ = procDestroyWindow.Call(h)
			return 0
		case wmDestroy:
			_, _, _ = procPostQuitMessage.Call(0)
			return 0
		}
		ret, _, _ := procDefWindowProcW.Call(h, uintptr(m), wParam, lParam)
		return ret
	})

	wc := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		lpfnWndProc:   wndProc,
		hInstance:     hInst,
		lpszClassName: clsName,
	}
	_, _, _ = procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	hwnd, _, _ = procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(clsName)), 0, 0, 0, 0, 0, 0, 0, 0, hInst, 0)
	if hwnd == 0 {
		return
	}

	hIcon = createVelocityTrayIcon()
	nid := notifyIconDataW{
		cbSize:           uint32(unsafe.Sizeof(notifyIconDataW{})),
		hWnd:             hwnd,
		uID:              1,
		uFlags:           nifMsg | nifIcon | nifTip,
		uCallbackMessage: wmTrayCallback,
		hIcon:            hIcon,
	}
	tipUTF16, _ := syscall.UTF16FromString(tooltip)
	copy(nid.szTip[:len(nid.szTip)-1], tipUTF16)
	_, _, _ = procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&nid)))

	doneCh := make(chan struct{})
	defer close(doneCh)
	go func() {
		select {
		case <-ctx.Done():
			_, _, _ = procPostMessageW.Call(hwnd, wmClose, 0, 0)
		case <-doneCh:
		}
	}()

	var m msg
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(ret) <= 0 {
			break
		}
		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}

	_, _, _ = procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&nid)))
	if hIcon != 0 {
		_, _, _ = procDestroyIcon.Call(hIcon)
	}
	if onQuit != nil {
		onQuit()
	}
}

func showTrayMenu(hwnd uintptr, onOpen func()) {
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return
	}
	defer procDestroyMenu.Call(hMenu)

	openLbl, _ := syscall.UTF16PtrFromString("Open VelocityEngine")
	exitLbl, _ := syscall.UTF16PtrFromString("Exit VelocityEngine")
	_, _, _ = procAppendMenuW.Call(hMenu, mfString, cmdOpenBrowser, uintptr(unsafe.Pointer(openLbl)))
	_, _, _ = procAppendMenuW.Call(hMenu, mfSeparator, 0, 0)
	_, _, _ = procAppendMenuW.Call(hMenu, mfString, cmdExitApp, uintptr(unsafe.Pointer(exitLbl)))

	var pt point
	_, _, _ = procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	_, _, _ = procSetForegroundWnd.Call(hwnd)
	_, _, _ = procTrackPopupMenu.Call(hMenu, tpmBottomAlign|tpmLeftAlign|tpmRightButton, uintptr(pt.X), uintptr(pt.Y), 0, hwnd, 0)
	_, _, _ = procPostMessageW.Call(hwnd, wmNull, 0, 0)
	_ = onOpen
}
