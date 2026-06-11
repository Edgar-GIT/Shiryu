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
	ID     int
	Start  int64
	End    int64
	Path   string
	Done   int64
	Paused bool
}

type Manager struct {
	url          string
	totalSize    int64
	workers      int
	parts        []Part
	progress     *atomic.Int64
	paused       *atomic.Bool
	stopped      *atomic.Bool
	startTime    time.Time
	pausedTime   time.Duration
	partsMutex   sync.RWMutex
	lastPartSize map[int]int64
	partMutex    sync.RWMutex
}

func NewManager(url string, totalSize int64, workers int, sessionDir string) *Manager {
	mgr := &Manager{
		url:          url,
		totalSize:    totalSize,
		workers:      workers,
		progress:     &atomic.Int64{},
		paused:       &atomic.Bool{},
		stopped:      &atomic.Bool{},
		lastPartSize: make(map[int]int64),
	}

	chunkSize := totalSize / int64(workers)
	for i := 0; i < workers; i++ {
		start := int64(i) * chunkSize
		end := start + chunkSize - 1
		if i == workers-1 {
			end = totalSize - 1
		}

		mgr.parts = append(mgr.parts, Part{
			ID:    i,
			Start: start,
			End:   end,
			Path:  fmt.Sprintf("%s/part_%d.tmp", sessionDir, i),
			Done:  0,
		})
	}

	return mgr
}

func (m *Manager) Start() error {
	m.startTime = time.Now()
	var wg sync.WaitGroup
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	for i := range m.parts {
		wg.Add(1)
		go m.downloadPart(i, client, &wg)
	}

	wg.Wait()

	if m.stopped.Load() {
		return fmt.Errorf("download stopped")
	}

	return nil
}

func (m *Manager) downloadPart(partIdx int, client *http.Client, wg *sync.WaitGroup) {
	defer wg.Done()

	part := &m.parts[partIdx]

	for retries := 0; retries < 3; retries++ {
		if m.stopped.Load() {
			return
		}

		if err := m.downloadPartAttempt(part, client); err == nil {
			part.Done = part.End - part.Start + 1
			return
		}

		if retries < 2 {
			time.Sleep(time.Duration((retries+1)*2) * time.Second)
		}
	}
}

func (m *Manager) downloadPartAttempt(part *Part, client *http.Client) error {
	req, _ := http.NewRequest("GET", m.url, nil)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", part.Start+part.Done, part.End))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("unexpected status: %d", resp.StatusCode)
	}

	out, err := os.OpenFile(part.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer out.Close()

	buf := make([]byte, 64*1024)
	for {
		if m.stopped.Load() {
			return fmt.Errorf("download stopped")
		}

		for m.paused.Load() && !m.stopped.Load() {
			time.Sleep(100 * time.Millisecond)
		}

		n, err := resp.Body.Read(buf)
		if n > 0 {
			_, writeErr := out.Write(buf[:n])
			if writeErr != nil {
				return fmt.Errorf("write failed: %w", writeErr)
			}
			m.progress.Add(int64(n))
			part.Done += int64(n)
		}

		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read failed: %w", err)
		}
	}
}

func (m *Manager) Pause() {
	m.paused.Store(true)
}

func (m *Manager) Resume() {
	m.paused.Store(false)
}

func (m *Manager) Stop() {
	m.stopped.Store(true)
}

func (m *Manager) GetProgress() int64 {
	return m.progress.Load()
}

func (m *Manager) GetSpeed() float64 {
	elapsed := time.Since(m.startTime).Seconds()
	if elapsed == 0 {
		return 0
	}
	return float64(m.progress.Load()) / 1024 / 1024 / elapsed
}

func (m *Manager) GetETA() time.Duration {
	speed := m.GetSpeed()
	if speed <= 0 {
		return 0
	}

	remaining := m.totalSize - m.progress.Load()
	secondsRemaining := float64(remaining) / 1024 / 1024 / speed
	return time.Duration(secondsRemaining) * time.Second
}

func (m *Manager) GetPartProgress() map[int]*PartProgress {
	m.partsMutex.RLock()
	defer m.partsMutex.RUnlock()

	result := make(map[int]*PartProgress)
	for i := range m.parts {
		part := m.parts[i]
		totalPart := part.End - part.Start + 1
		percentage := 0.0
		if totalPart > 0 {
			percentage = float64(part.Done) / float64(totalPart) * 100
		}

		result[i] = &PartProgress{
			ID:         i,
			Done:       part.Done,
			Total:      totalPart,
			Percentage: percentage,
		}
	}

	return result
}

func (m *Manager) IsPaused() bool {
	return m.paused.Load()
}

func (m *Manager) IsStopped() bool {
	return m.stopped.Load()
}

type PartProgress struct {
	ID         int
	Done       int64
	Total      int64
	Percentage float64
}

func CalculateWorkers(totalSize int64) int {
	cpus := runtime.NumCPU()
	mb := float64(totalSize) / (1024 * 1024)

	workers := int(mb/10) + 1
	if workers > cpus {
		workers = cpus
	}
	if workers < 1 {
		workers = 1
	}

	return workers
}
