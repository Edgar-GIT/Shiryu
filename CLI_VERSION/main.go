package main

import (
	"os"
	"os/exec"
	"runtime"

	core "shiryu/CLI_VERSION/src/go"
	"shiryu/CLI_VERSION/src/go/ui"
)

func main() {
	clearTerminal()
	ui.PrintBanner()
	core.Start()
}

func clearTerminal() {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "linux", "darwin":
		cmd = exec.Command("clear")
	case "windows":
		cmd = exec.Command("cmd", "/c", "cls")
	default:
		return
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}
