package integrity

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func CalculateSHA256(filePath string) (string, error) {
	zigSrc := filepath.Join(".", "CLI_VERSION", "src", "zig", "hash.zig")

	if _, err := os.Stat(zigSrc); err != nil {
		return "", fmt.Errorf("hash.zig not found: %w", err)
	}

	cmd := exec.Command("zig", "run", zigSrc, "--", filePath)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("hash calculation failed: %w", err)
	}

	hash := strings.TrimSpace(string(output))
	return hash, nil
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
