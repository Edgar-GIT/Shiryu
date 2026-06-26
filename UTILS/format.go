package util

import "fmt"

func FormatBytes(size int64) string {
	if size < 0 {
		size = 0
	}
	const unit = 1024.0
	s := float64(size)
	suffixes := []string{"B", "KB", "MB", "GB", "TB"}
	for i, suffix := range suffixes {
		if s < unit || i == len(suffixes)-1 {
			if suffix == "B" {
				return fmt.Sprintf("%d %s", size, suffix)
			}
			return fmt.Sprintf("%.2f %s", s, suffix)
		}
		s /= unit
	}
	return fmt.Sprintf("%d B", size)
}

func FormatDuration(seconds float64) string {
	if seconds < 0 {
		seconds = 0
	}
	h := int(seconds) / 3600
	m := (int(seconds) % 3600) / 60
	s := int(seconds) % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
	}
	return fmt.Sprintf("%02d:%02d", m, s)
}

func FormatSpeed(mbps float64) string {
	if mbps < 0.01 {
		return "0.00 MB/s"
	}
	return fmt.Sprintf("%.2f MB/s", mbps)
}

func ProgressBar(pct float64, width int) string {
	if width < 1 {
		width = 30
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(pct / 100 * float64(width))
	if filled > width {
		filled = width
	}
	bar := BarFill + repeat('#', filled) + BarEmpty + repeat(' ', width-filled) + Reset
	return bar
}

func repeat(c byte, n int) string {
	if n <= 0 {
		return ""
	}
	b := make([]byte, n)
	for i := range b {
		b[i] = c
	}
	return string(b)
}
