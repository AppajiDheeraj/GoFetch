package gofetch

import "concurrent_downloader/internal/events"

// Re-exported event types for consumers.
type ProgressMsg = events.ProgressMsg
type BatchProgressMsg = events.BatchProgressMsg
type DownloadStartedMsg = events.DownloadStartedMsg
type DownloadCompleteMsg = events.DownloadCompleteMsg
type DownloadErrorMsg = events.DownloadErrorMsg
type DownloadQueuedMsg = events.DownloadQueuedMsg
type DownloadPausedMsg = events.DownloadPausedMsg
type DownloadResumedMsg = events.DownloadResumedMsg
type DownloadRemovedMsg = events.DownloadRemovedMsg
