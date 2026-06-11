package main

import (
	"fmt"
	core "shiryu/CLI_VERSION/src/go"
	utils "shiryu/UTILS"
)

func main(){
	utils.ClearTerm(utils.DetectOS())
	utils.PrintBanner()
	fmt.Sprintf("Download URL: ", utils.color.GREEN)
	core.Start()
}
