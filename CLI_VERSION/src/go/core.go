package core

import (
	"bufio"
	"fmt"
	"os"
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

	if err := performDownload(reader, logger); err != nil {
		ui.PrintError(fmt.Sprintf("Download failed: %v", err))
		logger.LogError(fmt.Sprintf("Download failed: %v", err))
		return
	}

	var checksumComputed string
	var checksumMatch bool
	if checkIntegrity {
		matches, computed, err := performIntegrityCheck(reader, logger)
		if err != nil {
			ui.PrintError(fmt.Sprintf("Integrity check failed: %v", err))
			logger.LogError(fmt.Sprintf("Integrity check failed: %v", err))
		}
		checksumComputed = computed
		checksumMatch = matches
	}

	logger.LogSummary(
		currentSession.Metadata.Filename,
		currentSession.Metadata.Size,
		currentSession.DownloadTime,
		currentSession.DownloadSpeed,
		currentSession.Workers,
		checksumComputed,
		checksumMatch,
	)

	ui.PrintSuccess("Download completed successfully")
	logger.LogInfo("Download completed successfully")
}

func promptURL(reader *bufio.Reader) string {
	fmt.Print(ui.ColorGreen + "Enter download URL: " + ui.ColorReset)
	url, _ := reader.ReadString('\n')
	return strings.TrimSpace(url)
}

func performDownload(reader *bufio.Reader, logger *ui.Logger) error {
	mgr := download.NewManager(
		currentSession.URL,
		currentSession.Metadata.Size,
		currentSession.Workers,
		currentSession.SessionDir,
	)

	downloadStartTime := time.Now()
	done := make(chan struct{})

	go func() {
		if err := mgr.Start(); err != nil {
			logger.LogError(fmt.Sprintf("Download failed: %v", err))
		}
		close(done)
	}()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			downloadDuration := time.Since(downloadStartTime)
			fmt.Printf("\r%s[================================] 100.00%% | Download Complete%s\n", ui.ColorGreen, ui.ColorReset)
			logger.LogInfo("Download completed")

			mergeStartTime := time.Now()
			finalPath, err := utils.MergeParts(
				currentSession.SessionDir,
				currentSession.Metadata.Filename,
				currentSession.Workers,
			)
			if err != nil {
				logger.LogError(fmt.Sprintf("Merge failed: %v", err))
				return err
			}
			mergeTime := time.Since(mergeStartTime)

			currentSession.FinalFilePath = finalPath
			currentSession.DownloadTime = downloadDuration
			currentSession.DownloadSpeed = mgr.GetSpeed()

			ui.PrintDownloadSummary(
				currentSession.Metadata.Filename,
				currentSession.Metadata.Size,
				downloadDuration,
				currentSession.DownloadSpeed,
				currentSession.Workers,
			)

			logger.LogInfo(fmt.Sprintf("Merge time: %v", mergeTime))
			return nil

		case <-ticker.C:
			progress := mgr.GetProgress()
			speed := mgr.GetSpeed()
			eta := mgr.GetETA()

			ui.UpdateProgress(
				progress,
				currentSession.Metadata.Size,
				currentSession.Workers,
				speed,
				eta,
			)
		}
	}
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
