package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"shiryu/CLI_VERSION/src/go/download"
	"shiryu/CLI_VERSION/src/go/integrity"
	"shiryu/CLI_VERSION/src/go/ui"
	"shiryu/CLI_VERSION/src/go/utils"
	util "shiryu/UTILS"
)

type downloadResult struct {
	stopped bool
	err     error
}

func runInteractiveDownload(inp *util.Input, expectedChecksum string) (bool, int64, error) {
	mgr := download.NewManager(
		currentSession.URL,
		currentSession.Metadata.Size,
		currentSession.Workers,
		currentSession.SessionDir,
	)
	start := time.Now()
	corruptionOffset := int64(-1)

	for {
		util.ClearScreen()
		ui.RenderDownloadScreen(0, mgr.TotalSize(), 0, 0, 0, false)

		result := launchDownload(mgr, inp)
		if result.stopped {
			return true, corruptionOffset, nil
		}

		if result.err != nil {
			if mgr.IsStopped() {
				return true, corruptionOffset, nil
			}
			if mgr.HasCorruption() || result.err.Error() == "corrupted" {
				off := mgr.FindCorruptionOffset()
				if off < 0 {
					off = mgr.GetProgress()
				}
				pct := float64(off) / float64(mgr.TotalSize()) * 100
				ui.ClearScreen()
				ui.PrintCorruptionPrompt(pct)

				switch strings.TrimSpace(inp.ReadLine()) {
				case "1":
					mgr.ResetAll()
					corruptionOffset = -1
					continue
				case "2":
					mgr.TruncateFrom(off)
					corruptionOffset = off
					continue
				default:
					mgr.ResetAll()
					corruptionOffset = -1
					continue
				}
			}
			return false, corruptionOffset, result.err
		}

		finalPath, err := utils.MergeParts(currentSession.SessionDir, currentSession.Metadata.Filename, currentSession.Workers)
		if err != nil {
			off := mgr.FindCorruptionOffset()
			if off >= 0 {
				pct := float64(off) / float64(mgr.TotalSize()) * 100
				ui.ClearScreen()
				ui.PrintCorruptionPrompt(pct)
				switch strings.TrimSpace(inp.ReadLine()) {
				case "1":
					utils.CleanupPartFiles(currentSession.SessionDir, currentSession.Workers)
					mgr.ResetAll()
					corruptionOffset = -1
					continue
				case "2":
					utils.CleanupPartFiles(currentSession.SessionDir, currentSession.Workers)
					mgr.TruncateFrom(off)
					corruptionOffset = off
					continue
				}
			}
			return false, corruptionOffset, fmt.Errorf("merge failed: %w", err)
		}

		currentSession.FinalFilePath = finalPath
		end := time.Now()
		sizeBytes := fileSizeOrDefault(finalPath, currentSession.Metadata.Size)
		currentSession.DownloadTime = end.Sub(start)
		currentSession.DownloadSpeed = averageSpeedMBps(sizeBytes, currentSession.DownloadTime)
		createDownloaderLog(currentSession.SessionDir, start, end, currentSession.Workers, currentSession.Metadata.Filename, sizeBytes, expectedChecksum)
		return false, corruptionOffset, nil
	}
}

func launchDownload(mgr *download.Manager, inp *util.Input) downloadResult {
	cmdCh := make(chan string, 8)
	doneCh := make(chan error, 1)
	var runID atomic.Int32

	inp.StartDownload(cmdCh)
	defer inp.StopDownload()

	startRun := func() {
		id := runID.Add(1)
		go func(rid int32) {
			err := mgr.Start()
			if runID.Load() == rid {
				doneCh <- err
			}
		}(id)
	}
	startRun()

	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case cmd := <-cmdCh:
			switch cmd {
			case "pause", "p":
				mgr.Pause()
			case "continue", "resume", "c":
				mgr.Resume()
			case "restart", "r":
				mgr.ResetAll()
				startRun()
			case "stop", "s", "quit", "exit":
				mgr.Stop()
				return downloadResult{stopped: true}
			}
		case err := <-doneCh:
			if err != nil && err.Error() == "stopped" && mgr.IsRunning() {
				continue
			}
			return downloadResult{err: err}
		case <-ticker.C:
			ui.RenderDownloadScreen(
				mgr.GetProgress(),
				mgr.TotalSize(),
				mgr.GetSpeed(),
				mgr.Elapsed(),
				mgr.GetETA(),
				mgr.IsPaused(),
			)
		}
	}
}

func showFinalAssessment(finalPath string, expectedChecksum string, corruptionOffset int64) {
	assess := integrity.Assess(finalPath, currentSession.Metadata.Size, expectedChecksum, corruptionOffset)
	currentSession.Checksum = assess.Checksum
	ui.PrintIntegrityTrust(assess.Integrity, assess.Trust)
}

func cleanupOnStop() {
	if currentSession == nil {
		return
	}
	finalPath := filepath.Join(currentSession.SessionDir, currentSession.Metadata.Filename)
	os.Remove(finalPath)
	utils.CleanupPartFiles(currentSession.SessionDir, currentSession.Workers)
}
