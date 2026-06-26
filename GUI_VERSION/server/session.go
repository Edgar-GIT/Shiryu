package server

import (
	"fmt"
	"os"
	"sync"
	"time"

	"shiryu/CLI_VERSION/src/go/download"
	"shiryu/CLI_VERSION/src/go/integrity"
	"shiryu/CLI_VERSION/src/go/ui"
	"shiryu/CLI_VERSION/src/go/utils"
)

type State string

const (
	StateIdle        State = "idle"
	StateDownloading State = "downloading"
	StatePaused      State = "paused"
	StateCompleted   State = "completed"
	StateFailed      State = "failed"
	StateStopped     State = "stopped"
	StateCorruption  State = "corruption"
	StateMerging     State = "merging"
)

type Progress struct {
	State             State   `json:"state"`
	Progress          int64   `json:"progress"`
	Total             int64   `json:"total"`
	Percent           float64 `json:"percent"`
	Speed             float64 `json:"speed"`
	Elapsed           float64 `json:"elapsed"`
	ETA               float64 `json:"eta"`
	Paused            bool    `json:"paused"`
	Filename          string  `json:"filename"`
	Workers           int     `json:"workers"`
	Integrity         float64 `json:"integrity"`
	Trust             float64 `json:"trust"`
	OutputPath        string  `json:"outputPath"`
	Error             string  `json:"error,omitempty"`
	CorruptionPercent float64 `json:"corruptionPercent,omitempty"`
}

type StartRequest struct {
	URL            string `json:"url"`
	UseThreads     bool   `json:"useThreads"`
	CheckIntegrity bool   `json:"checkIntegrity"`
	Checksum       string `json:"checksum"`
	ClearTarget    bool   `json:"clearTarget"`
}

type Session struct {
	mu               sync.RWMutex
	state            State
	url              string
	metadata         *utils.URLMetadata
	sessionDir       string
	mgr              *download.Manager
	workers          int
	expectedChecksum string
	logger           *ui.Logger
	finalPath        string
	corruptionOffset int64
	startTime        time.Time
	downloadTime     time.Duration
	downloadSpeed    float64
	errMsg           string
}

type SessionManager struct {
	mu      sync.Mutex
	session *Session
}

func NewSessionManager() *SessionManager {
	return &SessionManager{}
}

func (sm *SessionManager) Current() *Session {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	return sm.session
}

func (sm *SessionManager) Start(req StartRequest) (*Session, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if sm.session != nil && (sm.session.state == StateDownloading || sm.session.state == StatePaused || sm.session.state == StateMerging) {
		return nil, fmt.Errorf("download already in progress")
	}
	metadata, err := utils.FetchURLMetadata(req.URL)
	if err != nil {
		return nil, err
	}
	if req.ClearTarget {
		_ = utils.ClearTarget()
	}
	sessionDir, err := utils.CreateSessionDirectory()
	if err != nil {
		return nil, err
	}
	logger, err := ui.NewLogger(sessionDir)
	if err != nil {
		return nil, err
	}
	workers := 1
	if req.UseThreads && metadata.SupportsRange {
		workers = download.CalculateWorkers(metadata.Size)
	}
	s := &Session{
		state:            StateDownloading,
		url:              req.URL,
		metadata:         metadata,
		sessionDir:       sessionDir,
		workers:          workers,
		expectedChecksum: req.Checksum,
		logger:           logger,
		startTime:        time.Now(),
		corruptionOffset: -1,
	}
	s.mgr = download.NewManager(req.URL, metadata.Size, workers, sessionDir)
	sm.session = s
	go sm.runDownload(s)
	return s, nil
}

func (sm *SessionManager) runDownload(s *Session) {
	err := s.mgr.Start()

	s.mu.Lock()
	if s.mgr.IsStopped() {
		s.state = StateStopped
		s.logger.LogInfo("Download stopped by user")
		s.logger.Close()
		s.mu.Unlock()
		return
	}
	if err != nil && (s.mgr.HasCorruption() || err.Error() == "corrupted") {
		s.state = StateCorruption
		s.errMsg = "corruption detected"
		s.mu.Unlock()
		return
	}
	if err != nil {
		s.state = StateFailed
		s.errMsg = err.Error()
		s.logger.LogError(err.Error())
		s.logger.Close()
		s.mu.Unlock()
		return
	}
	s.state = StateMerging
	s.mu.Unlock()

	finalPath, mergeErr := utils.MergeParts(s.sessionDir, s.metadata.Filename, s.workers)

	s.mu.Lock()
	defer s.mu.Unlock()
	if mergeErr != nil {
		if s.mgr.FindCorruptionOffset() >= 0 {
			s.state = StateCorruption
			s.errMsg = mergeErr.Error()
			return
		}
		s.state = StateFailed
		s.errMsg = mergeErr.Error()
		s.logger.LogError(mergeErr.Error())
		s.logger.Close()
		return
	}

	end := time.Now()
	s.finalPath = finalPath
	s.downloadTime = end.Sub(s.startTime)
	size := fileSizeOrDefault(finalPath, s.metadata.Size)
	s.downloadSpeed = speedMBps(size, s.downloadTime)
	assess := integrity.Assess(finalPath, s.metadata.Size, s.expectedChecksum, s.corruptionOffset)
	s.state = StateCompleted
	s.logger.LogSummary(s.metadata.Filename, size, s.downloadTime, s.downloadSpeed, s.workers, assess.Checksum, assess.Matches)
	s.logger.Close()
	recordStat(s.metadata.Filename, size, s.downloadTime, s.downloadSpeed, s.workers, assess.Integrity, assess.Trust)
}

func (sm *SessionManager) Control(action string) error {
	sm.mu.Lock()
	s := sm.session
	sm.mu.Unlock()
	if s == nil || s.mgr == nil {
		return fmt.Errorf("no active session")
	}
	s.mu.Lock()
	switch action {
	case "pause":
		s.mgr.Pause()
		s.state = StatePaused
	case "resume":
		s.mgr.Resume()
		s.state = StateDownloading
	case "restart":
		s.mgr.ResetAll()
		s.state = StateDownloading
		s.mu.Unlock()
		go sm.runDownload(s)
		return nil
	case "stop":
		s.mgr.Stop()
		s.state = StateStopped
	default:
		s.mu.Unlock()
		return fmt.Errorf("unknown action")
	}
	s.mu.Unlock()
	return nil
}

func (sm *SessionManager) Recover(action string) error {
	sm.mu.Lock()
	s := sm.session
	sm.mu.Unlock()
	if s == nil || s.mgr == nil {
		return fmt.Errorf("no active session")
	}
	s.mu.Lock()
	off := s.mgr.FindCorruptionOffset()
	if off < 0 {
		off = s.mgr.GetProgress()
	}
	switch action {
	case "restart":
		s.mgr.ResetAll()
		s.corruptionOffset = -1
	case "resume":
		s.mgr.TruncateFrom(off)
		s.corruptionOffset = off
	default:
		s.mu.Unlock()
		return fmt.Errorf("unknown action")
	}
	s.state = StateDownloading
	s.mu.Unlock()
	go sm.runDownload(s)
	return nil
}

func (s *Session) Snapshot() Progress {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := Progress{State: s.state, Workers: s.workers}
	if s.metadata != nil {
		p.Filename = s.metadata.Filename
	}
	if s.mgr != nil {
		total := s.mgr.TotalSize()
		prog := s.mgr.GetProgress()
		p.Progress = prog
		p.Total = total
		if total > 0 {
			p.Percent = float64(prog) / float64(total) * 100
		}
		p.Speed = s.mgr.GetSpeed()
		p.Elapsed = s.mgr.Elapsed().Seconds()
		p.ETA = s.mgr.GetETA().Seconds()
		p.Paused = s.mgr.IsPaused()
	}
	if s.state == StateCorruption && s.mgr != nil {
		p.CorruptionPercent = s.mgr.CorruptionPercent()
		if p.CorruptionPercent < 0 {
			off := s.mgr.FindCorruptionOffset()
			if off >= 0 && s.metadata != nil && s.metadata.Size > 0 {
				p.CorruptionPercent = float64(off) / float64(s.metadata.Size) * 100
			}
		}
		p.Error = s.errMsg
	}
	if s.state == StateCompleted && s.finalPath != "" && s.metadata != nil {
		assess := integrity.Assess(s.finalPath, s.metadata.Size, s.expectedChecksum, s.corruptionOffset)
		p.Integrity = assess.Integrity
		p.Trust = assess.Trust
		p.OutputPath = s.finalPath
		p.Elapsed = s.downloadTime.Seconds()
		p.Speed = s.downloadSpeed
		p.Percent = 100
		p.Progress = s.metadata.Size
		p.Total = s.metadata.Size
	}
	if s.state == StateFailed {
		p.Error = s.errMsg
	}
	return p
}

func fileSizeOrDefault(path string, fallback int64) int64 {
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return info.Size()
	}
	return fallback
}

func speedMBps(size int64, d time.Duration) float64 {
	sec := d.Seconds()
	if sec <= 0 {
		return 0
	}
	return (float64(size) / (1024 * 1024)) / sec
}
