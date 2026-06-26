package download

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

type Part struct {
	ID    int
	Start int64
	End   int64
	Path  string
	Done  int64
}

type PartProgress struct {
	ID         int
	Done       int64
	Total      int64
	Percentage float64
}

type Manager struct {
	url        string
	totalSize  int64
	workers    int
	parts      []Part
	progress   atomic.Int64
	paused     atomic.Bool
	stopped    atomic.Bool
	running    atomic.Bool
	startTime  time.Time
	pauseStart time.Time
	pausedDur  time.Duration
	speedBytes atomic.Int64
	speedTime  atomic.Int64
	wg         sync.WaitGroup
	mu         sync.Mutex
	failedPart int
}

func NewManager(url string, totalSize int64, workers int, sessionDir string) *Manager {
	mgr := &Manager{
		url:       url,
		totalSize: totalSize,
		workers:   workers,
	}
	chunk := totalSize / int64(workers)
	for i := 0; i < workers; i++ {
		start := int64(i) * chunk
		end := start + chunk - 1
		if i == workers-1 {
			end = totalSize - 1
		}
		path := fmt.Sprintf("%s/part_%d.tmp", sessionDir, i)
		done := int64(0)
		if info, err := os.Stat(path); err == nil {
			done = info.Size()
			if done > end-start+1 {
				done = end - start + 1
			}
		}
		mgr.parts = append(mgr.parts, Part{ID: i, Start: start, End: end, Path: path, Done: done})
		mgr.progress.Add(done)
	}
	return mgr
}

func (m *Manager) Start() error {
	m.stopped.Store(false)
	m.paused.Store(false)
	m.running.Store(true)
	m.startTime = time.Now()
	m.speedBytes.Store(0)
	m.speedTime.Store(time.Now().UnixNano())
	m.failedPart = -1

	tr := &http.Transport{
		MaxIdleConns:        1000,
		MaxIdleConnsPerHost: 1000,
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   true,
	}
	client := &http.Client{Transport: tr, Timeout: 0}

	for i := range m.parts {
		m.wg.Add(1)
		go m.downloadPart(i, client)
	}
	m.wg.Wait()
	m.running.Store(false)

	if m.stopped.Load() {
		return fmt.Errorf("stopped")
	}
	if m.failedPart >= 0 {
		return fmt.Errorf("corrupted")
	}
	return nil
}

func (m *Manager) downloadPart(idx int, client *http.Client) {
	defer m.wg.Done()
	part := &m.parts[idx]
	for attempt := 0; attempt < 3; attempt++ {
		if m.stopped.Load() {
			return
		}
		if err := m.downloadPartAttempt(part, client); err == nil {
			expected := part.End - part.Start + 1
			if part.Done != expected {
				m.mu.Lock()
				if m.failedPart < 0 || part.Start < m.parts[m.failedPart].Start {
					m.failedPart = idx
				}
				m.mu.Unlock()
			}
			return
		}
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * 2 * time.Second)
		}
	}
	m.mu.Lock()
	if m.failedPart < 0 || part.Start < m.parts[m.failedPart].Start {
		m.failedPart = idx
	}
	m.mu.Unlock()
}

func (m *Manager) downloadPartAttempt(part *Part, client *http.Client) error {
	req, err := http.NewRequest("GET", m.url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", part.Start+part.Done, part.End))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("status %d", resp.StatusCode)
	}

	out, err := os.OpenFile(part.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	buf := make([]byte, 64*1024)
	for {
		if m.stopped.Load() {
			return fmt.Errorf("stopped")
		}
		for m.paused.Load() && !m.stopped.Load() {
			time.Sleep(50 * time.Millisecond)
		}
		n, err := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := out.Write(buf[:n]); werr != nil {
				return werr
			}
			m.progress.Add(int64(n))
			part.Done += int64(n)
			m.speedBytes.Add(int64(n))
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

func (m *Manager) Pause() {
	if !m.paused.Load() {
		m.pauseStart = time.Now()
	}
	m.paused.Store(true)
}

func (m *Manager) Resume() {
	if m.paused.Load() {
		m.pausedDur += time.Since(m.pauseStart)
	}
	m.paused.Store(false)
}

func (m *Manager) Stop() {
	m.stopped.Store(true)
	m.paused.Store(false)
}

func (m *Manager) IsPaused() bool  { return m.paused.Load() }
func (m *Manager) IsStopped() bool { return m.stopped.Load() }
func (m *Manager) IsRunning() bool { return m.running.Load() }

func (m *Manager) GetProgress() int64 { return m.progress.Load() }
func (m *Manager) TotalSize() int64     { return m.totalSize }

func (m *Manager) GetSpeed() float64 {
	now := time.Now()
	last := time.Unix(0, m.speedTime.Load())
	elapsed := now.Sub(last).Seconds()
	if elapsed < 0.3 {
		elapsed = (now.Sub(m.startTime) - m.pausedDur).Seconds()
		if elapsed <= 0 {
			return 0
		}
		return float64(m.progress.Load()) / (1024 * 1024) / elapsed
	}
	bytes := m.speedBytes.Swap(0)
	m.speedTime.Store(now.UnixNano())
	if elapsed <= 0 {
		return 0
	}
	return float64(bytes) / (1024 * 1024) / elapsed
}

func (m *Manager) Elapsed() time.Duration {
	if m.paused.Load() {
		return time.Since(m.startTime) - m.pausedDur - time.Since(m.pauseStart)
	}
	return time.Since(m.startTime) - m.pausedDur
}

func (m *Manager) GetETA() time.Duration {
	speed := m.GetSpeed()
	if speed <= 0 {
		return 0
	}
	remaining := float64(m.totalSize-m.progress.Load()) / (1024 * 1024) / speed
	return time.Duration(remaining * float64(time.Second))
}

func (m *Manager) GetPartProgress() map[int]*PartProgress {
	result := make(map[int]*PartProgress, len(m.parts))
	for i := range m.parts {
		p := m.parts[i]
		total := p.End - p.Start + 1
		pct := 0.0
		if total > 0 {
			pct = float64(p.Done) / float64(total) * 100
		}
		result[i] = &PartProgress{ID: i, Done: p.Done, Total: total, Percentage: pct}
	}
	return result
}

func (m *Manager) HasCorruption() bool {
	return m.failedPart >= 0 || m.FindCorruptionOffset() >= 0
}

func (m *Manager) FindCorruptionOffset() int64 {
	for i := range m.parts {
		p := &m.parts[i]
		expected := p.End - p.Start + 1
		info, err := os.Stat(p.Path)
		if err != nil {
			if p.Done > 0 {
				return p.Start
			}
			continue
		}
		size := info.Size()
		if size < expected && m.progress.Load() >= m.totalSize {
			return p.Start + size
		}
		if size > expected {
			return p.Start + expected
		}
	}
	if m.progress.Load() < m.totalSize && !m.running.Load() && !m.stopped.Load() {
		return m.progress.Load()
	}
	return -1
}

func (m *Manager) CorruptionPercent() float64 {
	off := m.FindCorruptionOffset()
	if off < 0 {
		if m.failedPart >= 0 {
			return float64(m.parts[m.failedPart].Start) / float64(m.totalSize) * 100
		}
		return -1
	}
	return float64(off) / float64(m.totalSize) * 100
}

func (m *Manager) ResetAll() {
	m.Stop()
	for m.running.Load() {
		time.Sleep(20 * time.Millisecond)
	}
	m.stopped.Store(false)
	m.paused.Store(false)
	m.pausedDur = 0
	m.failedPart = -1
	m.progress.Store(0)
	for i := range m.parts {
		os.Remove(m.parts[i].Path)
		m.parts[i].Done = 0
	}
}

func (m *Manager) TruncateFrom(offset int64) {
	m.Stop()
	for m.running.Load() {
		time.Sleep(20 * time.Millisecond)
	}
	m.stopped.Store(false)
	m.paused.Store(false)
	m.pausedDur = 0
	m.failedPart = -1
	var total int64
	for i := range m.parts {
		p := &m.parts[i]
		if offset <= p.Start {
			os.Remove(p.Path)
			p.Done = 0
		} else if offset <= p.End+1 {
			trunc := offset - p.Start
			os.Truncate(p.Path, trunc)
			p.Done = trunc
		}
		total += p.Done
	}
	m.progress.Store(total)
}

func CalculateWorkers(totalSize int64) int {
	cpus := runtime.NumCPU()
	workers := int(float64(totalSize)/(1024*1024)/10) + 1
	if workers > cpus {
		workers = cpus
	}
	if workers < 1 {
		workers = 1
	}
	return workers
}
