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
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func VerifyChecksum(filePath, expected string) (bool, string, error) {
	computed, err := CalculateSHA256(filePath)
	if err != nil {
		return false, "", err
	}
	if expected == "" {
		return true, computed, nil
	}
	return strings.EqualFold(computed, expected), computed, nil
}

func ChecksumSimilarity(a, b string) float64 {
	a = strings.ToLower(strings.TrimSpace(a))
	b = strings.ToLower(strings.TrimSpace(b))
	if a == "" || b == "" {
		return 0
	}
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	matched := 0
	for i := 0; i < minLen; i++ {
		if a[i] == b[i] {
			matched++
		}
	}
	return float64(matched) / float64(maxLen) * 100
}

type Assessment struct {
	Integrity float64
	Trust     float64
	Checksum  string
	Matches   bool
}

func Assess(filePath string, expectedSize int64, expectedChecksum string, corruptionOffset int64) Assessment {
	info, err := os.Stat(filePath)
	if err != nil {
		return Assessment{Integrity: 0, Trust: 20}
	}
	actualSize := info.Size()
	sizePct := float64(actualSize) / float64(expectedSize) * 100
	if sizePct > 100 {
		sizePct = 100
	}
	if corruptionOffset >= 0 && expectedSize > 0 {
		good := float64(corruptionOffset) / float64(expectedSize) * 100
		if good < sizePct {
			sizePct = good
		}
	}
	if strings.TrimSpace(expectedChecksum) == "" {
		trust := 45.0
		if actualSize == expectedSize {
			trust = 55.0
		}
		return Assessment{Integrity: sizePct, Trust: trust}
	}
	computed, err := CalculateSHA256(filePath)
	if err != nil {
		return Assessment{Integrity: sizePct, Trust: 30}
	}
	matches := strings.EqualFold(computed, expectedChecksum)
	if matches {
		return Assessment{Integrity: 100, Trust: 95, Checksum: computed, Matches: true}
	}
	sim := ChecksumSimilarity(computed, expectedChecksum)
	trust := 80.0
	if corruptionOffset >= 0 {
		trust = 85.0
	}
	return Assessment{Integrity: sim, Trust: trust, Checksum: computed, Matches: false}
}
