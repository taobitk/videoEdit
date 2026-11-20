//go:build windows

package syswin

import "unsafe"

type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  uintptr
	hbmColor uintptr
}

func createVelocityTrayIcon() uintptr {
	const size = 32
	pixels := make([]byte, size*size*4)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			off := (y*size + x) * 4
			dx, dy := x-16, y-16
			if dx*dx+dy*dy <= 15*15 {
				inPlay := x >= 11 && x <= 23 && y >= 9+(x-11)/2 && y <= 23-(x-11)/2
				if inPlay {
					pixels[off], pixels[off+1], pixels[off+2], pixels[off+3] = 255, 255, 255, 255
				} else {
					pixels[off], pixels[off+1], pixels[off+2], pixels[off+3] = 241, 102, 99, 255 // BGRA (#6366f1)
				}
			}
		}
	}
	hColor, _, _ := procCreateBitmap.Call(size, size, 1, 32, uintptr(unsafe.Pointer(&pixels[0])))
	if hColor == 0 {
		return 0
	}
	defer procDeleteObject.Call(hColor)
	hMask, _, _ := procCreateBitmap.Call(size, size, 1, 1, 0)
	if hMask == 0 {
		return 0
	}
	defer procDeleteObject.Call(hMask)
	ii := iconInfo{fIcon: 1, hbmMask: hMask, hbmColor: hColor}
	hIcon, _, _ := procCreateIconInd.Call(uintptr(unsafe.Pointer(&ii)))
	return hIcon
}
