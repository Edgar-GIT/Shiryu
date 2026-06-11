package main

import (
	core "shiryu/CLI_VERSION/src/go"
	utils "shiryu/UTILS"
)

func main(){
	utils.ClearTerm(utils.DetectOS())
	utils.PrintBanner()
	core.Start()
}
