//go:build windows

package util

import (
	"syscall"
	"unsafe"
)

var (
	kernel32         = syscall.NewLazyDLL("kernel32.dll")
	procGetStdHandle = kernel32.NewProc("GetStdHandle")
	procPeek         = kernel32.NewProc("PeekConsoleInputW")
)

const stdInputHandle = ^uintptr(9)

type inputRecord struct {
	EventType uint16
	_         uint16
	_         uint16
	Padding   [16]uint16
}

func StdinReady() bool {
	handle, _, _ := procGetStdHandle.Call(stdInputHandle)
	if handle == 0 || handle == ^uintptr(0) {
		return false
	}
	var count uint32
	var rec inputRecord
	r, _, _ := procPeek.Call(
		handle,
		uintptr(unsafe.Pointer(&rec)),
		1,
		uintptr(unsafe.Pointer(&count)),
		0,
	)
	return r != 0 && count > 0
}
