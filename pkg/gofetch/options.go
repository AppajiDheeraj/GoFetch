package gofetch

import "concurrent_downloader/internal/config"

// ClientOptions configures the embedded engine.
type ClientOptions struct {
	MaxConcurrentDownloads int
	Verbose                bool
	Settings               *config.Settings
	StatePath              string
	LogsDir                string
}

// DownloadOptions controls per-download behavior.
type DownloadOptions struct {
	Mirrors     []string
	Headers     map[string]string
	ForceSingle bool
}
