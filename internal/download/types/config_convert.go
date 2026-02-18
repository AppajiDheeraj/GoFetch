package types

import "concurrent_downloader/internal/config"

// ConvertRuntimeConfig converts the app-level RuntimeConfig to the engine-level RuntimeConfig.
func ConvertRuntimeConfig(rc *config.RuntimeConfig) *RuntimeConfig {
	return &RuntimeConfig{
		MaxConnectionsPerHost: rc.MaxConnectionsPerHost,
		UserAgent:             rc.UserAgent,
		ProxyURL:              rc.ProxyURL,
		SequentialDownload:    rc.SequentialDownload,
		MinChunkSize:          rc.MinChunkSize,
		WorkerBufferSize:      rc.WorkerBufferSize,
		ForceSingle:           rc.ForceSingle,
		MaxTaskRetries:        rc.MaxTaskRetries,
		SlowWorkerThreshold:   rc.SlowWorkerThreshold,
		SlowWorkerGracePeriod: rc.SlowWorkerGracePeriod,
		StallTimeout:          rc.StallTimeout,
		SpeedEmaAlpha:         rc.SpeedEmaAlpha,
	}
}
