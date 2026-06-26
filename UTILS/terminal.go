package util

import (
	"os"
	"os/exec"
	"runtime"
)

func ClearScreen() {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout = os.Stdout
		_ = cmd.Run()
		return
	}
	cmd := exec.Command("clear")
	cmd.Stdout = os.Stdout
	_ = cmd.Run()
}
