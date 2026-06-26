package ui

import (
	"fmt"
	"strings"
	"time"

	util "shiryu/UTILS"
)

func ClearScreen()  { util.ClearScreen() }
func PrintBanner() { util.PrintBanner() }

func PromptUseThreads(available bool) bool {
	if !available {
		fmt.Println(util.Yellow + "NOTE: Server does not support parallel downloads (no Range header support)" + util.Reset)
		fmt.Println(util.Yellow + "Download will proceed sequentially" + util.Reset)
		return false
	}
	fmt.Print(util.Cyan + "Use multiple threads for faster download? [y/n]: " + util.Reset)
	var input string
	fmt.Scanln(&input)
	return strings.ToLower(input) == "y" || strings.ToLower(input) == "yes"
}

func PromptIntegrityCheck() bool {
	fmt.Print(util.Cyan + "Enable integrity check? [y/n]: " + util.Reset)
	var input string
	fmt.Scanln(&input)
	return strings.ToLower(input) == "y" || strings.ToLower(input) == "yes"
}

func PromptChecksum() string {
	fmt.Print(util.Yellow + "Enter expected SHA256 checksum (or press Enter to skip): " + util.Reset)
	var input string
	fmt.Scanln(&input)
	return input
}

func PrintFileInfo(filename string, size int64) {
	fmt.Printf("%sFile: %s%s\n", util.Yellow, filename, util.Reset)
	fmt.Printf("%sSize: %s%s\n", util.Yellow, util.FormatBytes(size), util.Reset)
}

func PrintThreadingInfo(available bool, workers int) {
	if !available {
		fmt.Println(util.Red + "✗ Threading unavailable (Range header unsupported)" + util.Reset)
	} else {
		fmt.Printf("%s✓ Threading enabled with %d workers%s\n", util.Green, workers, util.Reset)
	}
}

func RenderDownloadScreen(progress, total int64, speed float64, elapsed, eta time.Duration, paused bool) {
	util.ClearScreen()
	pct := 0.0
	if total > 0 {
		pct = float64(progress) / float64(total) * 100
	}
	bar := util.ProgressBar(pct, 30)
	status := util.Green + "DOWNLOADING"
	if paused {
		status = util.Yellow + "PAUSED"
	}
	fmt.Printf("\n%s%s%s\n", status, util.Reset, strings.Repeat(" ", 20))
	fmt.Printf("\n%s[%s%s] %s%.1f%%%s\n", util.Blue, bar, util.Blue, util.White, pct, util.Reset)
	fmt.Printf("\n%s%s%s / %s%s\n", util.Cyan, util.FormatBytes(progress), util.Dim, util.FormatBytes(total), util.Reset)
	fmt.Printf("\n%sSpeed:%s %s\n", util.Dim, util.Reset, util.FormatSpeed(speed))
	fmt.Printf("\n%sElapsed:%s %s  %s|  %sRemaining:%s %s\n",
		util.Dim, util.Reset, util.FormatDuration(elapsed.Seconds()),
		util.Dim, util.Dim, util.Reset, util.FormatDuration(eta.Seconds()))
	fmt.Println()
	fmt.Printf("  %s[CONTINUE]%s  %s[PAUSE]%s  %s[RESTART]%s  %s[STOP]%s\n",
		util.Green, util.Reset, util.Yellow, util.Reset, util.Magenta, util.Reset, util.Red, util.Reset)
	fmt.Printf("\n%sType a command:%s continue | pause | restart | stop\n", util.Dim, util.Reset)
}

func PrintDownloadSummary(filename string, size int64, duration time.Duration, speed float64, workers int, outputPath string) {
	fmt.Printf("\n%s=== Download Complete ===%s\n", util.Green, util.Reset)
	fmt.Printf("%sFile:%s %s\n", util.Yellow, util.Reset, filename)
	fmt.Printf("%sDownloaded Size:%s %s\n", util.Yellow, util.Reset, util.FormatBytes(size))
	fmt.Printf("%sElapsed Time:%s %s\n", util.Yellow, util.Reset, util.FormatDuration(duration.Seconds()))
	fmt.Printf("%sAverage Speed:%s %s\n", util.Yellow, util.Reset, util.FormatSpeed(speed))
	fmt.Printf("%sWorkers Used:%s %d\n", util.Yellow, util.Reset, workers)
	fmt.Printf("%sSaved To:%s %s\n", util.Yellow, util.Reset, outputPath)
}

func PrintIntegrityTrust(integrity, trust float64) {
	iColor := util.Green
	if integrity < 100 {
		iColor = util.Yellow
	}
	if integrity < 50 {
		iColor = util.Red
	}
	tColor := util.Green
	if trust < 70 {
		tColor = util.Yellow
	}
	if trust < 50 {
		tColor = util.Red
	}
	fmt.Printf("\n%sIntegrity:%s %s%.1f%%%s\n", util.Cyan, util.Reset, iColor, integrity, util.Reset)
	fmt.Printf("%sTrust:%s %s%.1f%%%s\n", util.Cyan, util.Reset, tColor, trust, util.Reset)
}

func PrintCorruptionPrompt(pct float64) {
	fmt.Printf("\n%s⚠ Corruption detected near %.1f%%%s\n", util.Red, pct, util.Reset)
	fmt.Printf("%s[1]%s Restart full download\n", util.Magenta, util.Reset)
	fmt.Printf("%s[2]%s Resume from corruption point (%.1f%%)\n", util.Cyan, util.Reset, pct)
	fmt.Print(util.Yellow + "Choose [1/2]: " + util.Reset)
}

func PrintError(msg string) {
	fmt.Printf("%s✗ Error: %s%s\n", util.Red, msg, util.Reset)
}

func PrintSuccess(msg string) {
	fmt.Printf("%s✓ %s%s\n", util.Green, msg, util.Reset)
}

func PrintInfo(msg string) {
	fmt.Printf("%sℹ %s%s\n", util.Cyan, msg, util.Reset)
}

func PrintWarning(msg string) {
	fmt.Printf("%s⚠ %s%s\n", util.Yellow, msg, util.Reset)
}
