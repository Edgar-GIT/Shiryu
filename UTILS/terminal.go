package utils

import (
	"os"
	"runtime"
)

func ClearTerminal() {
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		os.Stderr.WriteString("\033[2J\033[H")
	}
}
