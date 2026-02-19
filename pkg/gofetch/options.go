package gofetch

import "concurrent_downloader/internal/config"

// ClientOptions configures the embedded engine.
type ClientOptions struct {
	// MaxConcurrentDownloads overrides config to allow embedding apps to control load.
	MaxConcurrentDownloads int
	// Verbose enables additional diagnostic logging for host applications.
	Verbose bool
	// Settings supplies a prebuilt configuration to avoid reading from disk.
	Settings *config.Settings
	// StatePath isolates the embedded database from other GoFetch instances.
	StatePath string
	// LogsDir redirects log output when the host wants a custom location.
	LogsDir string
}

// DownloadOptions controls per-download behavior.
type DownloadOptions struct {
	// Mirrors provide alternate sources for the same file.
	Mirrors []string
	// Headers are forwarded to the HTTP client for custom auth or routing.
	Headers map[string]string
	// ForceSingle bypasses the concurrent downloader for servers that do not support ranges.
	ForceSingle bool
}
