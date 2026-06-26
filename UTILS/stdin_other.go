//go:build !linux && !darwin && !windows

package util

func StdinReady() bool {
	return true
}
