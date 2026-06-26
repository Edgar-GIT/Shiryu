package main

import (
	core "shiryu/CLI_VERSION/src/go"
	"shiryu/CLI_VERSION/src/go/ui"
)

func main() {
	ui.ClearScreen()
	ui.PrintBanner()
	core.Start()
}
