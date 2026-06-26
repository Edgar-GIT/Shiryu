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

		fmt.Print(ui.ColorYellow + "Delete existing downloads in target to free space? [y/n]: " + ui.ColorReset)
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
		logger.LogInfo(fmt.Sprintf("Threading enabled: %v", useThreads))
		logger.LogInfo(fmt.Sprintf("Workers: %d", workers))

		if err := performDownload(reader, logger, expectedChecksum); err != nil {
			ui.PrintError(fmt.Sprintf("Download failed: %v", err))
			logger.LogError(fmt.Sprintf("Download failed: %v", err))
			logger.Close()
			continue
		}

		finalPath := currentSession.FinalFilePath
		if finalPath == "" {
			finalPath = filepath.Join(currentSession.SessionDir, currentSession.Metadata.Filename)
			currentSession.FinalFilePath = finalPath
		}
		if absPath, err := filepath.Abs(finalPath); err == nil {
			finalPath = absPath
			currentSession.FinalFilePath = absPath
		}

		downloadedSize := fileSizeOrDefault(finalPath, currentSession.Metadata.Size)
		var checksumMatch bool
		var checksumErr error
		if strings.TrimSpace(expectedChecksum) != "" {
			currentSession.Checksum, checksumErr = integrity.CalculateSHA256(finalPath)
			if checksumErr != nil {
				logger.LogError("Failed to compute SHA256: " + checksumErr.Error())
			} else {
				checksumMatch = strings.EqualFold(currentSession.Checksum, expectedChecksum)
				logger.LogInfo("Computed SHA256: " + currentSession.Checksum)
				logger.LogInfo(fmt.Sprintf("Integrity check: %v", checksumMatch))
			}
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
		if strings.TrimSpace(expectedChecksum) != "" {
			if checksumErr != nil {
				ui.PrintWarning("SHA256 could not be calculated: " + checksumErr.Error())
			} else {
				percent := ui.ComputeChecksumSimilarity(currentSession.Checksum, expectedChecksum)
				ui.PrintIntegrityCheckEnhanced(currentSession.Checksum, expectedChecksum, checksumMatch, percent)
			}
		}

		if promptAfterDownload(reader) {
			ui.PrintInfo("Exiting...")
			return
		}

		ui.ClearScreen()
		ui.PrintBanner()
	}
}

func promptURL(reader *bufio.Reader) (string, string) {
	fmt.Print(ui.ColorGreen + "Enter download URL (or type 'exit' to quit, 'reset' to clear target): " + ui.ColorReset)
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
		fmt.Print(ui.ColorGreen + "\nPress Enter to return to the menu or type 'exit' to quit: " + ui.ColorReset)
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

func performDownload(_ *bufio.Reader, _ *ui.Logger, expectedChecksum string) error {
	aria2path, _ := exec.LookPath("aria2c")
	start := time.Now()
	if aria2path != "" {
		args := []string{
			"--console-log-level=error",
			"--summary-interval=0",
			"--show-console-readout=false",
			"--download-result=hide",
			"-x", strconv.Itoa(currentSession.Workers),
			"-s", strconv.Itoa(currentSession.Workers),
			"-k", "1M",
			"-d", currentSession.SessionDir,
			"-o", currentSession.Metadata.Filename,
			currentSession.URL,
		}
		cmd := exec.Command(aria2path, args...)
		cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C", "LANGUAGE=C")
		output, err := cmd.CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(output))
			if msg != "" {
				return fmt.Errorf("aria2c failed: %w: %s", err, msg)
			}
			return fmt.Errorf("aria2c failed: %w", err)
		}
		currentSession.FinalFilePath = filepath.Join(currentSession.SessionDir, currentSession.Metadata.Filename)
		recordDownloadStats(start, time.Now(), expectedChecksum)
		return nil
	}

	mgr := download.NewManager(currentSession.URL, currentSession.Metadata.Size, currentSession.Workers, currentSession.SessionDir)
	start = time.Now()
	if err := mgr.Start(); err != nil {
		return fmt.Errorf("download manager failed: %w", err)
	}
	finalPath, err := utils.MergeParts(currentSession.SessionDir, currentSession.Metadata.Filename, currentSession.Workers)
	if err != nil {
		return fmt.Errorf("merge failed: %w", err)
	}
	currentSession.FinalFilePath = finalPath
	recordDownloadStats(start, time.Now(), expectedChecksum)
	return nil
}

func createDownloaderLog(sessionDir string, start time.Time, end time.Time, workers int, filename string, sizeBytes int64, expectedChecksum string) {
	duration := end.Sub(start)
	durationSeconds := duration.Seconds()
	avg := averageSpeedMBps(sizeBytes, duration)
	logPath := filepath.Join(sessionDir, "downloader.log")
	f, err := os.Create(logPath)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "START_TIME: %d\n", start.UnixNano())
	fmt.Fprintf(f, "END_TIME: %d\n", end.UnixNano())
	fmt.Fprintf(f, "DURATION_SECONDS: %f\n", durationSeconds)
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
				fmt.Fprintln(f, "MESSAGE: checksum mismatch")
			}
		}
	}
}

func recordDownloadStats(start time.Time, end time.Time, expectedChecksum string) {
	sizeBytes := fileSizeOrDefault(currentSession.FinalFilePath, currentSession.Metadata.Size)
	currentSession.DownloadTime = end.Sub(start)
	currentSession.DownloadSpeed = averageSpeedMBps(sizeBytes, currentSession.DownloadTime)
	createDownloaderLog(
		currentSession.SessionDir,
		start,
		end,
		currentSession.Workers,
		currentSession.Metadata.Filename,
		sizeBytes,
		expectedChecksum,
	)
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
		percent := ui.ComputeChecksumSimilarity(computed, expectedChecksum)
		ui.PrintIntegrityCheckEnhanced(computed, expectedChecksum, matches, percent)
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
