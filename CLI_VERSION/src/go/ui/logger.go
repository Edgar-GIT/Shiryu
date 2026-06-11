package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Logger struct {
	filepath string
	file     *os.File
	enabled  bool
}

func NewLogger(sessionDir string) (*Logger, error) {
	logPath := filepath.Join(sessionDir, "download.log")
	file, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	return &Logger{
		filepath: logPath,
		file:     file,
		enabled:  true,
	}, nil
}

func (l *Logger) Log(msg string) {
	if !l.enabled || l.file == nil {
		return
	}

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	line := fmt.Sprintf("[%s] %s\n", timestamp, msg)
	l.file.WriteString(line)
}

func (l *Logger) LogError(msg string) {
	l.Log(fmt.Sprintf("[ERROR] %s", msg))
}

func (l *Logger) LogInfo(msg string) {
	l.Log(fmt.Sprintf("[INFO] %s", msg))
}

func (l *Logger) LogWarning(msg string) {
	l.Log(fmt.Sprintf("[WARNING] %s", msg))
}

func (l *Logger) LogSummary(filename string, sizeBytes int64, duration time.Duration, speed float64, workers int, checksum string, checksumMatch bool) {
	l.Log("=== DOWNLOAD SUMMARY ===")
	l.Log(fmt.Sprintf("File: %s", filename))
	l.Log(fmt.Sprintf("Size: %d bytes", sizeBytes))
	l.Log(fmt.Sprintf("Duration: %s", formatDuration(duration)))
	l.Log(fmt.Sprintf("Average Speed: %.2f MB/s", speed))
	l.Log(fmt.Sprintf("Workers Used: %d", workers))

	if checksum != "" {
		if checksumMatch {
			l.Log(fmt.Sprintf("Integrity: PASS (Checksum: %s)", checksum))
		} else {
			l.Log("Integrity: FAIL (Checksum mismatch)")
		}
	}
	l.Log("=== END SUMMARY ===")
}

func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}
