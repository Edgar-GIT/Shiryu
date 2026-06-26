package core

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"shiryu/CLI_VERSION/src/go/download"
	"shiryu/CLI_VERSION/src/go/integrity"
	"shiryu/CLI_VERSION/src/go/ui"
	"shiryu/CLI_VERSION/src/go/utils"
	util "shiryu/UTILS"
)

type DownloadSession struct {
	URL            string
	SessionDir     string
	Logger         *ui.Logger
	Metadata       *utils.URLMetadata
	Workers        int
	UseThreads     bool
	CheckIntegrity bool
	FinalFilePath  string
	DownloadTime   time.Duration
	DownloadSpeed  float64
	Checksum       string
}

var currentSession *DownloadSession

func Start() {
	reader := bufio.NewReader(os.Stdin)

	for {
		url, action := promptURL(reader)
		if action == "exit" {
			ui.PrintInfo("Exiting...")
			return
		}
		if action == "reset" {
			if err := utils.ClearTarget(); err != nil {
				ui.PrintError(fmt.Sprintf("Failed to clear target: %v", err))
			} else {
				ui.PrintWarning("Cleared existing downloads in target/")
			}
			continue
		}
		if url == "" {
			ui.PrintError("Invalid URL")
			continue
		}

		ui.PrintInfo(fmt.Sprintf("Fetching metadata: %s", url))
		metadata, err := utils.FetchURLMetadata(url)
		if err != nil {
			ui.PrintError(fmt.Sprintf("Failed to fetch metadata: %v", err))
			continue
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
			expectedChecksum = ui.PromptChecksum()
		}

		fmt.Print(util.Yellow + "Delete existing downloads in target to free space? [y/n]: " + util.Reset)
		delResp, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(delResp)) == "y" {
			if err := utils.ClearTarget(); err != nil {
				ui.PrintError(fmt.Sprintf("Failed to clear target: %v", err))
				continue
			}
			ui.PrintWarning("Cleared existing downloads in target/")
		}

		sessionDir, err := utils.CreateSessionDirectory()
		if err != nil {
			ui.PrintError(fmt.Sprintf("Failed to create session directory: %v", err))
			continue
		}

		logger, err := ui.NewLogger(sessionDir)
		if err != nil {
			ui.PrintError(fmt.Sprintf("Failed to create logger: %v", err))
			continue
		}

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
		logger.LogInfo(fmt.Sprintf("Workers: %d", workers))

		stopped, corruptionOffset, err := runInteractiveDownload(reader, expectedChecksum)
		if stopped {
			cleanupOnStop()
			logger.LogInfo("Download stopped by user")
			logger.Close()
			ui.ClearScreen()
			ui.PrintBanner()
			continue
		}
		if err != nil {
			ui.PrintError(fmt.Sprintf("Download failed: %v", err))
			logger.LogError(fmt.Sprintf("Download failed: %v", err))
			logger.Close()
			continue
		}

		finalPath := currentSession.FinalFilePath
		if absPath, err := filepath.Abs(finalPath); err == nil {
			finalPath = absPath
			currentSession.FinalFilePath = absPath
		}

		downloadedSize := fileSizeOrDefault(finalPath, currentSession.Metadata.Size)

		var checksumMatch bool
		if strings.TrimSpace(expectedChecksum) != "" && currentSession.Checksum != "" {
			checksumMatch = strings.EqualFold(currentSession.Checksum, expectedChecksum)
		}

		logger.LogSummary(
			currentSession.Metadata.Filename,
			downloadedSize,
			currentSession.DownloadTime,
			currentSession.DownloadSpeed,
			currentSession.Workers,
			currentSession.Checksum,
			checksumMatch,
		)
		logger.LogInfo("Download completed successfully")
		logger.Close()

		ui.ClearScreen()
		ui.PrintDownloadSummary(
			currentSession.Metadata.Filename,
			downloadedSize,
			currentSession.DownloadTime,
			currentSession.DownloadSpeed,
			currentSession.Workers,
			finalPath,
		)
		showFinalAssessment(finalPath, expectedChecksum, corruptionOffset)

		if promptAfterDownload(reader) {
			ui.PrintInfo("Exiting...")
			return
		}

		ui.ClearScreen()
		ui.PrintBanner()
	}
}

func promptURL(reader *bufio.Reader) (string, string) {
	fmt.Print(util.Green + "Enter download URL (or type 'exit' to quit, 'reset' to clear target): " + util.Reset)
	url, _ := reader.ReadString('\n')
	s := strings.TrimSpace(url)
	lower := strings.ToLower(s)
	if lower == "exit" {
		return "", "exit"
	}
	if lower == "reset" {
		return "", "reset"
	}
	return s, ""
}

func promptAfterDownload(reader *bufio.Reader) bool {
	for {
		fmt.Print(util.Green + "\nPress Enter to return to the menu or type 'exit' to quit: " + util.Reset)
		response, _ := reader.ReadString('\n')
		action := strings.ToLower(strings.TrimSpace(response))
		switch action {
		case "", "menu", "m", "back":
			return false
		case "exit", "quit", "q":
			return true
		default:
			ui.PrintWarning("Unknown option. Press Enter to return to the menu or type 'exit' to quit.")
		}
	}
}

func createDownloaderLog(sessionDir string, start, end time.Time, workers int, filename string, sizeBytes int64, expectedChecksum string) {
	duration := end.Sub(start)
	avg := averageSpeedMBps(sizeBytes, duration)
	logPath := filepath.Join(sessionDir, "downloader.log")
	f, err := os.Create(logPath)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "START_TIME: %d\n", start.UnixNano())
	fmt.Fprintf(f, "END_TIME: %d\n", end.UnixNano())
	fmt.Fprintf(f, "DURATION_SECONDS: %f\n", duration.Seconds())
	fmt.Fprintf(f, "AVERAGE_SPEED_MBPS: %f\n", avg)
	fmt.Fprintf(f, "WORKERS: %d\n", workers)
	fmt.Fprintf(f, "FILE: %s\n", filename)
	fmt.Fprintf(f, "SIZE_BYTES: %d\n", sizeBytes)
	if expectedChecksum != "" {
		computed, err := integrity.CalculateSHA256(filepath.Join(sessionDir, filename))
		if err == nil {
			fmt.Fprintf(f, "CHECKSUM: %s\n", computed)
			if strings.EqualFold(computed, expectedChecksum) {
				fmt.Fprintln(f, "CHECKSUM_MATCH: true")
				fmt.Fprintln(f, "STATUS: OK")
			} else {
				fmt.Fprintln(f, "CHECKSUM_MATCH: false")
				fmt.Fprintln(f, "STATUS: FAIL")
			}
		}
	}
}

func fileSizeOrDefault(path string, fallback int64) int64 {
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return info.Size()
	}
	return fallback
}

func averageSpeedMBps(sizeBytes int64, duration time.Duration) float64 {
	seconds := duration.Seconds()
	if seconds <= 0 {
		return 0
	}
	return (float64(sizeBytes) / (1024.0 * 1024.0)) / seconds
}
