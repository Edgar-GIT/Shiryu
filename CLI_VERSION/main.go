package main

import (
	utils "shiryu/UTILS"
	core  "shiryu/CLI_VERSION/src/go"
)

func main(){
	utils.ClearTerm(utils.DetectOS())
	utils.PrintBanner()
}
