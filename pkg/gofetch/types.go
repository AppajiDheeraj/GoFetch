package gofetch

import (
	"concurrent_downloader/internal/config"
	"concurrent_downloader/internal/download/types"
)

// Re-exported types for the public API.
type Settings = config.Settings

type DownloadStatus = types.DownloadStatus
type DownloadEntry = types.DownloadEntry
type DownloadState = types.DownloadState
type ProgressState = types.ProgressState
type RuntimeConfig = types.RuntimeConfig

type AddOptions = types.AddOptions

var ErrPaused = types.ErrPaused
