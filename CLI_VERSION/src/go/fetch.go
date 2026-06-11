package core

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
)
type URLInfo struct {
	Filename     string
	SizeMB       float64
	DownloadLink string
}

var fileInfo URLInfo

func prepare(url string) bool{
	resp, err := http.Head(url)
	if err != nil {
		fmt.Printf("\033[31mError fetching URL info: %v\033[0m\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("\033[31mUnable to access URL (status %d)\033[0m\n", resp.StatusCode)
		return
	}

	filename := path.Base(url)
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		const prefix = "filename="
		if idx := strings.Index(cd, prefix); idx != -1 {
			filename = strings.Trim(cd[idx+len(prefix):], "\"')")
		}
	}

	sizeBytes := int64(0)
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		if sz, err := strconv.ParseInt(cl, 10, 64); err == nil {
			sizeBytes = sz
		}
	}
	sizeMB := float64(sizeBytes) / (1024 * 1024)

	fileInfo = URLInfo{Filename: filename, SizeMB: sizeMB, DownloadLink: url}

	fmt.Printf("\033[33mFile: %s\n", fileInfo.Filename)
	fmt.Printf("Size: %.2f MB\n", fileInfo.SizeMB)
	fmt.Printf("Link: %s\033[0m\n", fileInfo.DownloadLink)

	if fileInfo.SizeMB <= 0 {
		fmt.Printf("\033[31mCannot determine file size – aborting download.\033[0m\n")
		return false
	}
	fmt.Printf("\033[32mDownload is ready to start.\033[0m\n")
	return true
}

func Start() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\033[32mDownload URL: \033[0m")
	url, _ := reader.ReadString('\n')
	url = strings.TrimSpace(url)

	if url != "" {fmt.Printf("\033[34mFetching -> %s\033[0m\n", url)}

	if (prepare(url)){
		fmt.Print("Do you want to download with integrity check? [y/n]")
	}
	

}


