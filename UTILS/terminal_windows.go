//go:build windows

package util

import (
	"os"
	"syscall"
	"unsafe"
)

func EnableColors() {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	setMode := kernel32.NewProc("SetConsoleMode")
	getMode := kernel32.NewProc("GetConsoleMode")
	handle := syscall.Handle(os.Stdout.Fd())
	var mode uint32
	r1, _, _ := getMode.Call(uintptr(handle), uintptr(unsafe.Pointer(&mode)))
	if r1 == 0 {
		return
	}
	const enableVirtualTerminal = 0x0004
	_, _, _ = setMode.Call(uintptr(handle), uintptr(mode|enableVirtualTerminal))
}
