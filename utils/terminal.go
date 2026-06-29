package utils

import (
	"syscall"
	"unsafe"
)

func GetTerminalWidth() int {
	type winsize struct {
		Row    uint16
		Col    uint16
		Xpixel uint16
		Ypixel uint16
	}
	ws := &winsize{}
	// syscall.TIOCGWINSZ gets window size of system stdout
	retCode, _, _ := syscall.Syscall(syscall.SYS_IOCTL,
		uintptr(syscall.Stdout),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)
	if int(retCode) == -1 || ws.Col == 0 {
		return 60
	}
	return int(ws.Col)
}
