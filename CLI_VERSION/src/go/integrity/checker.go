package integrity

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

func CalculateSHA256(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func VerifyChecksum(filePath string, expectedChecksum string) (bool, string, error) {
	computed, err := CalculateSHA256(filePath)
	if err != nil {
		return false, "", err
	}

	if expectedChecksum == "" {
		return true, computed, nil
	}

	matches := strings.EqualFold(computed, expectedChecksum)
	return matches, computed, nil
}
