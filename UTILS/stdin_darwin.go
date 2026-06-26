//go:build darwin

package util

import (
	"os"
	"syscall"
	"unsafe"
)

const fionread = 0x4004667f

func StdinReady() bool {
	var n int32
	_, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		os.Stdin.Fd(),
		fionread,
		uintptr(unsafe.Pointer(&n)),
	)
	return err == 0 && n > 0
}
