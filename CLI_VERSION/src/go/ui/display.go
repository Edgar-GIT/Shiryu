package ui

import (
	"fmt"
	"strings"
	"time"
)

const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorCyan    = "\033[36m"
	ColorMagenta = "\033[35m"
)

func PrintBanner() {
	banner := `
 $$$$$$\  $$\       $$\                               
$$  __$$\ $$ |      \__|                              
$$ /  \__|$$$$$$$\  $$\  $$$$$$\  $$\   $$\ $$\   $$\ 
\$$$$$$\  $$  __$$\ $$ |$$  __$$\ $$ |  $$ |$$ |  $$ |
 \____$$\ $$ |  $$ |$$ |$$ |      $$ |  $$ |$$ |  $$ |
$$\   $$ |$$ |  $$ |$$ |$$ |      $$ |  $$ |$$ |  $$ |
\$$$$$$  |$$ |  $$ |$$ |$$ |      \$$$$$$$ |\$$$$$$  |
 \______/ \__|  \__|\__|\__|       \____$$ | \______/ 
                                  $$\   $$ |          
                                  \$$$$$$  |          
                                   \______/           `

	lines := strings.Split(banner, "\n")
	colors := []string{
		"\033[38;5;26m", "\033[38;5;27m", "\033[38;5;32m", "\033[38;5;33m", "\033[38;5;39m",
		"\033[38;5;38m", "\033[38;5;44m", "\033[38;5;43m", "\033[38;5;49m", "\033[38;5;48m",
		"\033[38;5;49m", "\033[38;5;43m", "\033[38;5;44m",
	}

	for i, line := range lines {
		color := colors[i%len(colors)]
		fmt.Printf("%s%s%s\n", color, line, ColorReset)
	}

	box := fmt.Sprintf(`%s+-----------------------------------------------------------+
| %sA web, ultra fast, download booster%s | %sversion: 2.0.0%s      |
|                    %sGithub: Edgar-GIT%s                      |
+-----------------------------------------------------------+%s
`,
		"\033[38;5;205m",
		"\033[38;5;51m",
		"\033[38;5;205m",
		"\033[38;5;255m",
		"\033[38;5;205m",
		"\033[38;5;51m",
		"\033[38;5;205m",
		ColorReset)
	fmt.Println(box)
}

func PromptURL() (string, error) {
	fmt.Print(ColorGreen + "Enter download URL: " + ColorReset)
	var url string
	_, err := fmt.Scanln(&url)
	return url, err
}

func PromptUseThreads(available bool) bool {
	if !available {
		fmt.Println(ColorYellow + "NOTE: Server does not support parallel downloads (no Range header support)" + ColorReset)
		fmt.Println(ColorYellow + "Download will proceed sequentially" + ColorReset)
		return false
	}

	fmt.Print(ColorCyan + "Use multiple threads for faster download? [y/n]: " + ColorReset)
	var input string
	fmt.Scanln(&input)
	return strings.ToLower(input) == "y" || strings.ToLower(input) == "yes"
}

func PromptIntegrityCheck() bool {
	fmt.Print(ColorCyan + "Enable integrity check? [y/n]: " + ColorReset)
	var input string
	fmt.Scanln(&input)
	return strings.ToLower(input) == "y" || strings.ToLower(input) == "yes"
}

func PromptChecksum() string {
	fmt.Print(ColorYellow + "Enter expected SHA256 checksum (or press Enter to skip): " + ColorReset)
	var input string
	fmt.Scanln(&input)
	return input
}

func PrintFileInfo(filename string, sizeBytes int64) {
	sizeMB := float64(sizeBytes) / (1024 * 1024)
	var sizeStr string

	if sizeMB > 1024 {
		sizeStr = fmt.Sprintf("%.2f GB", sizeMB/1024)
	} else {
		sizeStr = fmt.Sprintf("%.2f MB", sizeMB)
	}

	fmt.Printf("%sFile: %s%s\n", ColorYellow, filename, ColorReset)
	fmt.Printf("%sSize: %s%s\n", ColorYellow, sizeStr, ColorReset)
}

func PrintThreadingInfo(available bool, workers int) {
	if !available {
		fmt.Println(ColorRed + "✗ Threading unavailable (Range header unsupported)" + ColorReset)
	} else {
		fmt.Printf("%s✓ Threading enabled with %d workers%s\n", ColorGreen, workers, ColorReset)
	}
}

func UpdateProgress(progress int64, total int64, threadCount int, speed float64, eta time.Duration) {
	percentage := float64(progress) / float64(total) * 100
	statusBar := buildProgressBar(percentage)

	fmt.Printf("\r%s[%s] %.2f%% | Speed: %.2f MB/s | Threads: %d | ETA: %s%s",
		ColorBlue, statusBar, percentage, speed, threadCount, formatDuration(eta), ColorReset)
}

func UpdateDetailedProgress(progress int64, total int64, threadProgresses map[int]interface{}, speed float64, eta time.Duration) {
	percentage := float64(progress) / float64(total) * 100
	statusBar := buildProgressBar(percentage)

	fmt.Printf("\r%s[%s] %.2f%% | Speed: %.2f MB/s | ETA: %s%s",
		ColorBlue, statusBar, percentage, speed, formatDuration(eta), ColorReset)

	fmt.Printf("\n%sThread Progress:%s\n", ColorCyan, ColorReset)
	for id := range threadProgresses {
		fmt.Printf("  Thread %d: processing...\n", id)
	}
}

func buildProgressBar(percentage float64) string {
	width := 30
	filled := int(percentage / 100 * float64(width))
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("=", filled) + strings.Repeat("-", width-filled)
	return bar
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%02d:%02d", minutes, seconds)
}

func PrintDownloadSummary(filename string, sizeBytes int64, duration time.Duration, speed float64, workers int) {
	sizeMB := float64(sizeBytes) / (1024 * 1024)
	var sizeStr string

	if sizeMB > 1024 {
		sizeStr = fmt.Sprintf("%.2f GB", sizeMB/1024)
	} else {
		sizeStr = fmt.Sprintf("%.2f MB", sizeMB)
	}

	fmt.Printf("\n%s=== Download Summary ===%s\n", ColorGreen, ColorReset)
	fmt.Printf("%sFile:%s %s\n", ColorYellow, ColorReset, filename)
	fmt.Printf("%sSize:%s %s\n", ColorYellow, ColorReset, sizeStr)
	fmt.Printf("%sTime:%s %s\n", ColorYellow, ColorReset, formatDuration(duration))
	fmt.Printf("%sAverage Speed:%s %.2f MB/s\n", ColorYellow, ColorReset, speed)
	fmt.Printf("%sWorkers Used:%s %d\n", ColorYellow, ColorReset, workers)
}

func PrintIntegrityCheck(computed string, expected string, matches bool) {
	if matches {
		fmt.Printf("%s✓ Checksum match - file integrity confirmed%s\n", ColorGreen, ColorReset)
	} else {
		fmt.Printf("%s✗ Checksum mismatch%s\n", ColorRed, ColorReset)
		fmt.Printf("%sComputed: %s%s\n", ColorYellow, computed, ColorReset)
		fmt.Printf("%sExpected: %s%s\n", ColorYellow, expected, ColorReset)
	}
}

func PrintError(msg string) {
	fmt.Printf("%s✗ Error: %s%s\n", ColorRed, msg, ColorReset)
}

func PrintSuccess(msg string) {
	fmt.Printf("%s✓ %s%s\n", ColorGreen, msg, ColorReset)
}

func PrintInfo(msg string) {
	fmt.Printf("%sℹ %s%s\n", ColorCyan, msg, ColorReset)
}

func PrintWarning(msg string) {
	fmt.Printf("%s⚠ %s%s\n", ColorYellow, msg, ColorReset)
}
