package gofetch

import (
	"concurrent_downloader/internal/config"
	"concurrent_downloader/internal/core"
	"concurrent_downloader/internal/download"
	"concurrent_downloader/internal/download/types"
	"concurrent_downloader/internal/state"
	"concurrent_downloader/internal/utils"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

// Client exposes a stable API for embedding GoFetch.
type Client struct {
	service    core.DownloadService
	pool       *download.WorkerPool
	progressCh chan any

	settings  *config.Settings
	statePath string
	logsDir   string

	closeOnce sync.Once
}

// NewClient initializes the engine and returns a ready-to-use client.
func NewClient(opts *ClientOptions) (*Client, error) {
	settings := resolveSettings(opts)
	if settings == nil {
		return nil, errors.New("settings not available")
	}

	if err := config.EnsureDirs(); err != nil {
		return nil, err
	}

	logsDir := config.GetLogsDir()
	if opts != nil && opts.LogsDir != "" {
		logsDir = opts.LogsDir
	}
	if logsDir != "" {
		if err := os.MkdirAll(logsDir, 0o755); err != nil {
			return nil, err
		}
	}

	utils.ConfigureDebug(logsDir)
	if opts != nil {
		utils.SetVerbose(opts.Verbose)
	}
	utils.CleanupLogs(settings.General.LogRetentionCount)

	statePath := filepath.Join(config.GetStateDir(), "GoFetch.db")
	if opts != nil && opts.StatePath != "" {
		statePath = opts.StatePath
	}
	if err := os.MkdirAll(filepath.Dir(statePath), 0o755); err != nil {
		return nil, err
	}
	state.Configure(statePath)

	maxDownloads := settings.Network.MaxConcurrentDownloads
	if maxDownloads <= 0 {
		maxDownloads = settings.Connections.MaxConcurrentDownloads
	}
	if opts != nil && opts.MaxConcurrentDownloads > 0 {
		maxDownloads = opts.MaxConcurrentDownloads
	}

	progressCh := make(chan any, 100)
	pool := download.NewWorkerPool(progressCh, maxDownloads)
	service := core.NewLocalDownloadServiceWithInput(pool, progressCh)

	return &Client{
		service:    service,
		pool:       pool,
		progressCh: progressCh,
		settings:   settings,
		statePath:  statePath,
		logsDir:    logsDir,
	}, nil
}

func resolveSettings(opts *ClientOptions) *config.Settings {
	if opts != nil && opts.Settings != nil {
		return opts.Settings
	}
	settings, err := config.LoadSettings()
	if err != nil {
		return config.DefaultSettings()
	}
	return settings
}

// Service exposes the underlying download service.
func (c *Client) Service() core.DownloadService {
	if c == nil {
		return nil
	}
	return c.service
}

// Add queues a download and returns its ID.
func (c *Client) Add(url string, outputDir string, filename string, opts *DownloadOptions) (string, error) {
	if c == nil || c.service == nil {
		return "", errors.New("client not initialized")
	}

	var mirrors []string
	var headers map[string]string
	var addOpts *types.AddOptions

	if opts != nil {
		mirrors = opts.Mirrors
		headers = opts.Headers
		if opts.ForceSingle {
			addOpts = &types.AddOptions{ForceSingle: true}
		}
	}

	return c.service.Add(url, outputDir, filename, mirrors, headers, addOpts)
}

// List returns current download statuses.
func (c *Client) List() ([]types.DownloadStatus, error) {
	if c == nil || c.service == nil {
		return nil, errors.New("client not initialized")
	}
	return c.service.List()
}

// History returns completed downloads.
func (c *Client) History() ([]types.DownloadEntry, error) {
	if c == nil || c.service == nil {
		return nil, errors.New("client not initialized")
	}
	return c.service.History()
}

// Pause pauses an active download.
func (c *Client) Pause(id string) error {
	if c == nil || c.service == nil {
		return errors.New("client not initialized")
	}
	return c.service.Pause(id)
}

// Resume resumes a paused download.
func (c *Client) Resume(id string) error {
	if c == nil || c.service == nil {
		return errors.New("client not initialized")
	}
	return c.service.Resume(id)
}

// ResumeBatch resumes multiple paused downloads.
func (c *Client) ResumeBatch(ids []string) []error {
	if c == nil || c.service == nil {
		return []error{errors.New("client not initialized")}
	}
	return c.service.ResumeBatch(ids)
}

// Delete cancels and removes a download.
func (c *Client) Delete(id string) error {
	if c == nil || c.service == nil {
		return errors.New("client not initialized")
	}
	return c.service.Delete(id)
}

// GetStatus returns the status of a single download.
func (c *Client) GetStatus(id string) (*types.DownloadStatus, error) {
	if c == nil || c.service == nil {
		return nil, errors.New("client not initialized")
	}
	return c.service.GetStatus(id)
}

// StreamEvents subscribes to the live event stream.
func (c *Client) StreamEvents(ctx context.Context) (<-chan interface{}, func(), error) {
	if c == nil || c.service == nil {
		return nil, nil, errors.New("client not initialized")
	}
	return c.service.StreamEvents(ctx)
}

// Publish injects an event into the stream (advanced use).
func (c *Client) Publish(msg interface{}) error {
	if c == nil || c.service == nil {
		return errors.New("client not initialized")
	}
	return c.service.Publish(msg)
}

// Shutdown gracefully stops the client and releases resources.
func (c *Client) Shutdown() error {
	if c == nil {
		return nil
	}
	var err error
	c.closeOnce.Do(func() {
		if c.service != nil {
			err = c.service.Shutdown()
		}
		state.CloseDB()
	})
	return err
}
