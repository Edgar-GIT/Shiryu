package utils

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

var colors = map[string]string{
	"RESET":  "\033[0m",
	"RED":    "\033[31m",
	"GREEN":  "\033[32m",
	"BLUE":   "\033[34m",
	"GRAY":   "\033[37m",
	"FADE1":  "\033[38;5;26m",
	"FADE2":  "\033[38;5;27m",
	"FADE3":  "\033[38;5;32m",
	"FADE4":  "\033[38;5;33m",
	"FADE5":  "\033[38;5;39m",
	"FADE6":  "\033[38;5;38m",
	"FADE7":  "\033[38;5;44m",
	"FADE8":  "\033[38;5;43m",
	"FADE9":  "\033[38;5;49m",
	"FADE10": "\033[38;5;48m",
	"PINK":   "\033[38;5;205m",
	"WHITE":  "\033[38;5;255m",
	"CYAN":   "\033[38;5;51m",
}

func DetectOS() string{
	return runtime.GOOS
}

func ClearTerm(osName string){
	var cmd *exec.Cmd
	switch osName {
	case "linux", "darwin":
		cmd = exec.Command("clear")
	case "windows":
		cmd = exec.Command("cls")
	default:
		return
	}
	cmd.Stdout = os.Stdout
	cmd.Run()
}

func PrintBanner(){
	banner:= `
 $$$$$$\  $$\       $$\                               
$$  __$$\ $$ |      \__|                              
$$ /  \__|$$$$$$$\  $$\  $$$$$$\  $$\   $$\ $$\   $$\ 
\$$$$$$\  $$  __$$\ $$ |$$  __$$\ $$ |  $$ |$$ |  $$ |
 \____$$\ $$ |  $$ |$$ |$$ |  \__|$$ |  $$ |$$ |  $$ |
$$\   $$ |$$ |  $$ |$$ |$$ |      $$ |  $$ |$$ |  $$ |
\$$$$$$  |$$ |  $$ |$$ |$$ |      \$$$$$$$ |\$$$$$$  |
 \______/ \__|  \__|\__|\__|       \____$$ | \______/ 
                                  $$\   $$ |          
                                  \$$$$$$  |          
                                   \______/           `

	fadeSequence := []string{
		colors["FADE1"], colors["FADE2"], colors["FADE3"], colors["FADE4"], colors["FADE5"],
		colors["FADE6"], colors["FADE7"], colors["FADE8"], colors["FADE9"], colors["FADE10"],
		colors["FADE9"], colors["FADE8"], colors["FADE7"],
	}

	lines := strings.Split(banner, "\n")
	for i, line := range lines {
		color := fadeSequence[i%len(fadeSequence)]
		fmt.Printf("%s%s%s\n", color, line, colors["RESET"])
	}

	box := fmt.Sprintf(`
%s+-----------------------------------------------------------+
| %sA web, ultra fast, download booster%s | %sversion: 1.0.0%s    |
| %sGithub: Edgar-GIT%s                                         |
+-----------------------------------------------------------+%s
`, colors["PINK"], colors["CYAN"], colors["PINK"], colors["WHITE"], colors["PINK"], colors["CYAN"], colors["PINK"], colors["RESET"])
	fmt.Println(box)
}
