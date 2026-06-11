package main

import (
	utils "shiryu/UTILS"
)

func main(){
	utils.ClearTerm(utils.DetectOS())
	utils.PrintBanner()
}
