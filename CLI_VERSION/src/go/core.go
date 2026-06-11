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

		// compute and display SHA256 for final file
		finalPath := filepath.Join(currentSession.SessionDir, currentSession.Metadata.Filename)
		computed, cerr := integrity.CalculateSHA256(finalPath)
		if cerr != nil {
			ui.PrintWarning("Failed to compute SHA256: " + cerr.Error())
			logger.LogError("Failed to compute SHA256: " + cerr.Error())
		} else {
			ui.PrintInfo("Computed SHA256: " + computed)
			logger.LogInfo("Computed SHA256: " + computed)
		}

		// ingest downloader.log if present and show final integrity result
		zlog := filepath.Join(currentSession.SessionDir, "downloader.log")
		if data, err := os.ReadFile(zlog); err == nil {
			logger.LogInfo("Downloader log:\n" + string(data))
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
			logger.LogSummary(
				currentSession.Metadata.Filename,
				currentSession.Metadata.Size,
				currentSession.DownloadTime,
				currentSession.DownloadSpeed,
				currentSession.Workers,
				currentSession.Checksum,
				match,
			)

			percent := ui.ComputeChecksumSimilarity(checksum, expectedChecksum)
			ui.PrintIntegrityCheckEnhanced(checksum, expectedChecksum, match, percent)
		} else {
			if computed != "" {
				percent := ui.ComputeChecksumSimilarity(computed, expectedChecksum)
				ui.PrintIntegrityCheckEnhanced(computed, expectedChecksum, strings.EqualFold(computed, expectedChecksum), percent)
				logger.LogInfo(fmt.Sprintf("Integrity check: %v", strings.EqualFold(computed, expectedChecksum)))
			}
		}

		ui.PrintSuccess("Download completed successfully")
		logger.LogInfo("Download completed successfully")
		logger.Close()
		// loop back for next download
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

func performDownload(reader *bufio.Reader, logger *ui.Logger, expectedChecksum string) error {
	aria2path, _ := exec.LookPath("aria2c")
	start := time.Now()
	if aria2path != "" {
		args := []string{"-x", strconv.Itoa(currentSession.Workers), "-s", strconv.Itoa(currentSession.Workers), "-k", "1M", "-d", currentSession.SessionDir, "-o", currentSession.Metadata.Filename, currentSession.URL}
		cmd := exec.Command(aria2path, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("aria2c failed: %w", err)
		}
		currentSession.FinalFilePath = filepath.Join(currentSession.SessionDir, currentSession.Metadata.Filename)
		createDownloaderLog(currentSession.SessionDir, start, time.Now(), currentSession.Workers, currentSession.Metadata.Filename, currentSession.Metadata.Size, expectedChecksum)
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
	createDownloaderLog(currentSession.SessionDir, start, time.Now(), currentSession.Workers, currentSession.Metadata.Filename, currentSession.Metadata.Size, expectedChecksum)
	return nil
}

func createDownloaderLog(sessionDir string, start time.Time, end time.Time, workers int, filename string, sizeBytes int64, expectedChecksum string) {
	duration := end.Sub(start).Seconds()
	avg := (float64(sizeBytes) / (1024.0 * 1024.0)) / duration
	logPath := filepath.Join(sessionDir, "downloader.log")
	f, err := os.Create(logPath)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "START_TIME: %d\n", start.UnixNano())
	fmt.Fprintf(f, "END_TIME: %d\n", end.UnixNano())
	fmt.Fprintf(f, "DURATION_SECONDS: %f\n", duration)
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
	} else {
		computed, err := integrity.CalculateSHA256(filepath.Join(sessionDir, filename))
		if err == nil {
			fmt.Fprintf(f, "CHECKSUM: %s\n", computed)
			fmt.Fprintln(f, "CHECKSUM_MATCH: true")
			fmt.Fprintln(f, "STATUS: OK")
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
