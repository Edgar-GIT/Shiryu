package core

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func Start() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\033[32mDownload URL: \033[0m")
	url, _ := reader.ReadString('\n')
	url = strings.TrimSpace(url)

	if url != "" {
		fmt.Printf("\033[34mTarget: %s\033[0m\n", url)
	}
}


