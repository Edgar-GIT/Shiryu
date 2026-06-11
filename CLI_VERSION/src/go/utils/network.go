package utils

import (
	"fmt"
	"net/http"
	"path"
	"strconv"
	"strings"
)

type URLMetadata struct {
	Filename      string
	Size          int64
	SupportsRange bool
}

func FetchURLMetadata(url string) (*URLMetadata, error) {
	resp, err := http.Head(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	filename := extractFilename(resp.Header.Get("Content-Disposition"), url)
	size := extractContentLength(resp.Header.Get("Content-Length"))

	if size <= 0 {
		return nil, fmt.Errorf("unable to determine file size")
	}

	supportsRange := resp.Header.Get("Accept-Ranges") == "bytes"

	return &URLMetadata{
		Filename:      filename,
		Size:          size,
		SupportsRange: supportsRange,
	}, nil
}

func extractFilename(disposition string, url string) string {
	if disposition != "" {
		const prefix = "filename="
		if idx := strings.Index(disposition, prefix); idx != -1 {
			filename := strings.Trim(disposition[idx+len(prefix):], "\"')")
			if filename != "" {
				return filename
			}
		}
	}
	return path.Base(url)
}

func extractContentLength(cl string) int64 {
	if cl == "" {
		return 0
	}
	size, _ := strconv.ParseInt(cl, 10, 64)
	return size
}
