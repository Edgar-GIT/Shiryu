package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type DownloadStat struct {
	Filename  string    `json:"filename"`
	Size      int64     `json:"size"`
	Duration  float64   `json:"duration"`
	Speed     float64   `json:"speed"`
	Workers   int       `json:"workers"`
	Integrity float64   `json:"integrity"`
	Trust     float64   `json:"trust"`
	Timestamp time.Time `json:"timestamp"`
}

type StatsSummary struct {
	TotalDownloads int            `json:"totalDownloads"`
	TotalBytes     int64          `json:"totalBytes"`
	TotalDuration  float64        `json:"totalDuration"`
	AvgSpeed       float64        `json:"avgSpeed"`
	Recent         []DownloadStat `json:"recent"`
}

var (
	statsMu sync.Mutex
	stats   []DownloadStat
)

func recordStat(filename string, size int64, duration time.Duration, speed float64, workers int, integrity, trust float64) {
	statsMu.Lock()
	defer statsMu.Unlock()
	stats = append(stats, DownloadStat{
		Filename:  filename,
		Size:      size,
		Duration:  duration.Seconds(),
		Speed:     speed,
		Workers:   workers,
		Integrity: integrity,
		Trust:     trust,
		Timestamp: time.Now(),
	})
	if len(stats) > 50 {
		stats = stats[len(stats)-50:]
	}
	saveStats()
}

func loadStats() {
	path := statsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	statsMu.Lock()
	defer statsMu.Unlock()
	_ = json.Unmarshal(data, &stats)
}

func saveStats() {
	data, err := json.Marshal(stats)
	if err != nil {
		return
	}
	_ = os.MkdirAll(filepath.Dir(statsPath()), 0755)
	_ = os.WriteFile(statsPath(), data, 0644)
}

func statsPath() string {
	return filepath.Join(".", "target", "gui_stats.json")
}

func GetStats() StatsSummary {
	loadFromDisk()
	statsMu.Lock()
	defer statsMu.Unlock()
	s := StatsSummary{Recent: make([]DownloadStat, len(stats))}
	copy(s.Recent, stats)
	s.TotalDownloads = len(stats)
	for _, st := range stats {
		s.TotalBytes += st.Size
		s.TotalDuration += st.Duration
		s.AvgSpeed += st.Speed
	}
	if s.TotalDownloads > 0 {
		s.AvgSpeed /= float64(s.TotalDownloads)
	}
	return s
}

func loadFromDisk() {
	path := statsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	statsMu.Lock()
	defer statsMu.Unlock()
	var loaded []DownloadStat
	if json.Unmarshal(data, &loaded) == nil {
		stats = loaded
	}
}
