package util

import (
	"fmt"
	"strings"
)

func PrintBanner() {
	banner := `
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
	fades := []string{Fade1, Fade2, Fade3, Fade4, Fade5, Fade6, Fade7, Fade8, Fade9, Fade10, Fade9, Fade8, Fade7}
	lines := strings.Split(banner, "\n")
	for i, line := range lines {
		fmt.Printf("%s%s%s\n", fades[i%len(fades)], line, Reset)
	}
	fmt.Printf(`%s+-----------------------------------------------------------+
| %sA web, ultra fast, download booster%s | %sversion: 2.0.0%s      |
|                    %sGithub: Edgar-GIT%s                      |
+-----------------------------------------------------------+%s
`, Pink, Accent, Pink, White, Pink, Accent, Pink, Reset)
}
