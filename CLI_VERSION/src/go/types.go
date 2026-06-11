package core

type DownloadState int

const (
	StateIdle DownloadState = iota
	StateDownloading
	StatePaused
	StateCompleted
	StateFailed
)
