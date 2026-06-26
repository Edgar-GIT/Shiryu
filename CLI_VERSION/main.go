package main

import (
	core "shiryu/CLI_VERSION/src/go"
	util "shiryu/UTILS"
)

func main() {
	util.EnableColors()
	util.ClearScreen()
	util.PrintBanner()
	core.Start()
}
