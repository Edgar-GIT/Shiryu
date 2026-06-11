package core

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"shiryu/CLI_VERSION/src/go/download"
	"shiryu/CLI_VERSION/src/go/integrity"
	"shiryu/CLI_VERSION/src/go/ui"
	"shiryu/CLI_VERSION/src/go/utils"
)

type DownloadSession struct {
	URL            string
	SessionDir     string
	Logger         *ui.Logger
	Metadata       *utils.URLMetadata
	FinalFilePath  string
	DownloadTime   time.Duration
	DownloadSpeed  float64
	Workers        int
	UseThreads     bool
	CheckIntegrity bool
	Checksum       string
}

var currentSession *DownloadSession

func Start() {
	reader := bufio.NewReader(os.Stdin)

	url := promptURL(reader)
	if url == "" {
		ui.PrintError("Invalid URL")
		return
	}

	ui.PrintInfo(fmt.Sprintf("Fetching metadata: %s", url))

	metadata, err := utils.FetchURLMetadata(url)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to fetch metadata: %v", err))
		return
	}

	ui.PrintFileInfo(metadata.Filename, metadata.Size)

	useThreads := ui.PromptUseThreads(metadata.SupportsRange)
	workers := 1

	if useThreads && metadata.SupportsRange {
		workers = download.CalculateWorkers(metadata.Size)
	}

	ui.PrintThreadingInfo(metadata.SupportsRange, workers)

	checkIntegrity := ui.PromptIntegrityCheck()
	var expectedChecksum string
	if checkIntegrity {
		fmt.Print(ui.ColorYellow + "Provide expected SHA256 checksum now? [y/n]: " + ui.ColorReset)
		resp, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(resp)) == "y" {
			fmt.Print(ui.ColorYellow + "Enter SHA256 checksum: " + ui.ColorReset)
			cs, _ := reader.ReadString('\n')
			expectedChecksum = strings.TrimSpace(cs)
		}
	}

	fmt.Print(ui.ColorYellow + "Delete existing downloads in target to free space? [y/n]: " + ui.ColorReset)
	delResp, _ := reader.ReadString('\n')
	if strings.ToLower(strings.TrimSpace(delResp)) == "y" {
		if err := utils.ClearTarget(); err != nil {
			ui.PrintError(fmt.Sprintf("Failed to clear target: %v", err))
			return
		}
		ui.PrintWarning("Cleared existing downloads in target/")
	}

	sessionDir, err := utils.CreateSessionDirectory()
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to create session directory: %v", err))
		return
	}

	logger, err := ui.NewLogger(sessionDir)
	if err != nil {
		ui.PrintError(fmt.Sprintf("Failed to create logger: %v", err))
		return
	}
	defer logger.Close()

	currentSession = &DownloadSession{
		URL:            url,
		SessionDir:     sessionDir,
		Logger:         logger,
		Metadata:       metadata,
		Workers:        workers,
		UseThreads:     useThreads,
		CheckIntegrity: checkIntegrity,
	}

	logger.LogInfo(fmt.Sprintf("Starting download: %s", metadata.Filename))
	logger.LogInfo(fmt.Sprintf("File size: %d bytes", metadata.Size))
	logger.LogInfo(fmt.Sprintf("Threading enabled: %v", useThreads))
	logger.LogInfo(fmt.Sprintf("Workers: %d", workers))

	if err := performDownload(reader, logger, expectedChecksum); err != nil {
		ui.PrintError(fmt.Sprintf("Download failed: %v", err))
		logger.LogError(fmt.Sprintf("Download failed: %v", err))
		return
	}

	// ingest Zig downloader log and update session
	zlog := filepath.Join(currentSession.SessionDir, "downloader.log")
	if data, err := os.ReadFile(zlog); err == nil {
		logger.LogInfo("Zig downloader log:\n" + string(data))
		// parse duration and avg speed
		lines := strings.Split(string(data), "\n")
		var durS float64
		var avg float64
		var checksum string
		var match bool
		for _, L := range lines {
			if strings.HasPrefix(L, "DURATION_SECONDS:") {
				fmt.Sscanf(L, "DURATION_SECONDS: %f", &durS)
			}
			if strings.HasPrefix(L, "AVERAGE_SPEED_MBPS:") {
				fmt.Sscanf(L, "AVERAGE_SPEED_MBPS: %f", &avg)
			}
			if strings.HasPrefix(L, "CHECKSUM:") {
				parts := strings.SplitN(L, ":", 2)
				if len(parts) == 2 {
					checksum = strings.TrimSpace(parts[1])
				}
			}
			if strings.HasPrefix(L, "CHECKSUM_MATCH:") {
				parts := strings.SplitN(L, ":", 2)
				if len(parts) == 2 {
					match = strings.TrimSpace(parts[1]) == "true"
				}
			}
		}
		currentSession.DownloadTime = time.Duration(durS * float64(time.Second))
		currentSession.DownloadSpeed = avg
		currentSession.Checksum = checksum
		// log summary via Go logger as well
		logger.LogSummary(
			currentSession.Metadata.Filename,
			currentSession.Metadata.Size,
			currentSession.DownloadTime,
			currentSession.DownloadSpeed,
			currentSession.Workers,
			currentSession.Checksum,
			match,
		)
	}

	ui.PrintSuccess("Download completed successfully")
	logger.LogInfo("Download completed successfully")
}

func promptURL(reader *bufio.Reader) string {
	fmt.Print(ui.ColorGreen + "Enter download URL: " + ui.ColorReset)
	url, _ := reader.ReadString('\n')
	return strings.TrimSpace(url)
}

func performDownload(reader *bufio.Reader, logger *ui.Logger, expectedChecksum string) error {
	zigPath := filepath.Join(".", "CLI_VERSION", "src", "zig", "downloader.zig")
	args := []string{"run", zigPath, "--", currentSession.URL, currentSession.SessionDir, strconv.Itoa(currentSession.Workers)}
	if expectedChecksum != "" {
		args = append(args, expectedChecksum)
	}
	cmd := exec.Command("zig", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("zig downloader failed: %w", err)
	}
	currentSession.FinalFilePath = filepath.Join(currentSession.SessionDir, currentSession.Metadata.Filename)
	return nil
}

func performIntegrityCheck(reader *bufio.Reader, logger *ui.Logger) (bool, string, error) {
	fmt.Print(ui.ColorYellow + "Do you want to provide a checksum to verify? [y/n]: " + ui.ColorReset)
	response, _ := reader.ReadString('\n')

	var expectedChecksum string
	if strings.ToLower(strings.TrimSpace(response)) == "y" {
		fmt.Print(ui.ColorYellow + "Enter SHA256 checksum: " + ui.ColorReset)
		expectedChecksum, _ = reader.ReadString('\n')
		expectedChecksum = strings.TrimSpace(expectedChecksum)
	}

	ui.PrintInfo("Calculating SHA256...")
	logger.LogInfo("Starting integrity check")

	matches, computed, err := integrity.VerifyChecksum(currentSession.FinalFilePath, expectedChecksum)
	if err != nil {
		return false, "", err
	}

	if expectedChecksum == "" {
		ui.PrintInfo(fmt.Sprintf("Computed SHA256: %s", computed))
		logger.LogInfo(fmt.Sprintf("Computed SHA256: %s", computed))
	} else {
		ui.PrintIntegrityCheck(computed, expectedChecksum, matches)
		logger.LogInfo(fmt.Sprintf("Integrity check: %v", matches))

		if !matches {
			fmt.Print(ui.ColorRed + "Checksum mismatch! Delete downloaded file and retry? [y/n]: " + ui.ColorReset)
			response, _ := reader.ReadString('\n')
			if strings.ToLower(strings.TrimSpace(response)) == "y" {
				if err := os.Remove(currentSession.FinalFilePath); err != nil {
					logger.LogError(fmt.Sprintf("Failed to delete file: %v", err))
					return false, computed, err
				}
				ui.PrintWarning("File deleted. Please retry download.")
				logger.LogInfo("File deleted by user request")
				return false, computed, fmt.Errorf("file deleted due to checksum mismatch")
			}
		}
	}

	logger.LogInfo(fmt.Sprintf("Integrity check completed, matches: %v", matches))
	currentSession.Checksum = computed
	return matches, computed, nil
}
