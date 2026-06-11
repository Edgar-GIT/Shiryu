package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func CreateSessionDirectory() (string, error) {
	targetRoot := filepath.Join(".", "target")
	if err := os.MkdirAll(targetRoot, 0755); err != nil {
		return "", fmt.Errorf("failed to create target directory: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	sessionDir := filepath.Join(targetRoot, timestamp)

	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create session directory: %w", err)
	}

	return sessionDir, nil
}

func GetPartPath(sessionDir string, partID int) string {
	return filepath.Join(sessionDir, fmt.Sprintf("part_%d.tmp", partID))
}

func CleanupPartFiles(sessionDir string, numParts int) error {
	for i := 0; i < numParts; i++ {
		partPath := GetPartPath(sessionDir, i)
		if err := os.Remove(partPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove part file %d: %w", i, err)
		}
	}
	return nil
}

func MergeParts(sessionDir string, filename string, numParts int) (string, error) {
	destPath := filepath.Join(sessionDir, filename)
	dest, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("failed to create destination file: %w", err)
	}
	defer dest.Close()

	for i := 0; i < numParts; i++ {
		partPath := GetPartPath(sessionDir, i)
		part, err := os.Open(partPath)
		if err != nil {
			return "", fmt.Errorf("failed to open part %d: %w", i, err)
		}

		if _, err := dest.ReadFrom(part); err != nil {
			part.Close()
			return "", fmt.Errorf("failed to merge part %d: %w", i, err)
		}
		part.Close()

		if err := os.Remove(partPath); err != nil {
			return "", fmt.Errorf("failed to remove part file %d: %w", i, err)
		}
	}

	return destPath, nil
}

func CheckChecksum(filePath string) (string, error) {
	fi, err := os.Stat(filePath)
	if err != nil {
		return "", fmt.Errorf("file not found: %w", err)
	}
	if fi.IsDir() {
		return "", fmt.Errorf("path is a directory, not a file")
	}
	return filePath, nil
}
