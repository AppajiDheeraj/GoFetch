package concurrent

import (
	"concurrent_downloader/internal/download/types"
	"concurrent_downloader/internal/state"
	"concurrent_downloader/internal/utils"
	"context"
	"crypto/tls"
	"fmt"
	"math"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/quic-go/quic-go"
	"github.com/quic-go/quic-go/http3"
)

// ConcurrentDownloader handles multi-connection downloads and owns the
// in-memory coordination needed for pause/resume and health monitoring.
type ConcurrentDownloader struct {
	ProgressChan chan<- any           // Channel for events (start/complete/error)
	ID           string               // Download ID
	State        *types.ProgressState // Shared state for TUI polling
	activeTasks  map[int]*ActiveTask
	activeMu     sync.Mutex
	URL          string // For pause/resume
	DestPath     string // For pause/resume
	Runtime      *types.RuntimeConfig
	bufPool      sync.Pool
	Headers      map[string]string // Custom HTTP headers from browser (cookies, auth, etc.)
}

type protocolClient struct {
	name   string
	client *http.Client
}

type clientSet struct {
	primary        protocolClient
	fallbacks      []protocolClient
	http3Transport *http3.Transport
}

func (c *clientSet) Close() {
	if c == nil || c.http3Transport == nil {
		return
	}

	if err := c.http3Transport.Close(); err != nil {
		utils.Debug("Error closing HTTP/3 transport: %v", err)
	}
}

// NewConcurrentDownloader creates a new concurrent downloader with all required parameters.
func NewConcurrentDownloader(id string, progressCh chan<- any, progState *types.ProgressState, runtime *types.RuntimeConfig) *ConcurrentDownloader {
	if runtime == nil {
		runtime = &types.RuntimeConfig{
			MaxConnectionsPerHost: types.PerHostMax,
			MinChunkSize:          types.MinChunk,
			WorkerBufferSize:      types.WorkerBuffer,
			ProtocolPreference:    types.ProtocolAuto,
		}
	}

	return &ConcurrentDownloader{
		ID:           id,
		ProgressChan: progressCh,
		State:        progState,
		activeTasks:  make(map[int]*ActiveTask),
		Runtime:      runtime,
		bufPool: sync.Pool{
			New: func() any {
				// Use configured buffer size
				size := runtime.GetWorkerBufferSize()
				buf := make([]byte, size)
				return &buf
			},
		},
	}
}

// getInitialConnections returns the starting number of connections based on file size.
// The heuristic favors fewer connections for small files to avoid overhead.
func (d *ConcurrentDownloader) getInitialConnections(fileSize int64) int {
	maxConns := d.Runtime.GetMaxConnectionsPerHost()
	minChunkSize := d.Runtime.GetMinChunkSize() // e.g., 1MB or 5MB

	if fileSize <= 0 {
		return 1
	}

	// 1. Calculate ideal workers using the Square Root heuristic
	// Convert to float first to avoid integer truncation on small files
	sizeMB := float64(fileSize) / (1024 * 1024)
	calculatedWorkers := int(math.Round(math.Sqrt(sizeMB)))

	// 2. Hard constraint: Don't create chunks smaller than MinChunkSize
	// If file is 20MB and MinChunk is 10MB, we strictly can't have more than 2 workers
	if minChunkSize > 0 {
		maxPossibleChunks := int(fileSize / minChunkSize)
		if maxPossibleChunks < 1 {
			maxPossibleChunks = 1
		}
		if calculatedWorkers > maxPossibleChunks {
			calculatedWorkers = maxPossibleChunks
		}
	}

	// 3. Safety Floors and Ceilings
	if calculatedWorkers < 1 {
		return 1
	}
	if calculatedWorkers > maxConns {
		return maxConns
	}

	return calculatedWorkers
}

// ReportMirrorError marks a mirror as having an error in the state
func (d *ConcurrentDownloader) ReportMirrorError(url string) {
	if d.State == nil {
		return
	}

	mirrors := d.State.GetMirrors()
	changed := false
	for i, m := range mirrors {
		if m.URL == url && !m.Error {
			mirrors[i].Error = true
			changed = true
			break
		}
	}

	if changed {
		d.State.SetMirrors(mirrors)
	}
}

// calculateChunkSize determines optimal chunk size while enforcing minimums.
func (d *ConcurrentDownloader) calculateChunkSize(fileSize int64, numConns int) int64 {
	// Safety check
	if numConns <= 0 {
		return d.Runtime.GetMinChunkSize() // Fallback
	}

	chunkSize := fileSize / int64(numConns)

	// Clamp to min from config (but not max - we want large chunks)
	minChunk := d.Runtime.GetMinChunkSize()

	if chunkSize < minChunk {
		chunkSize = minChunk
	}

	// Align to 4KB
	chunkSize = (chunkSize / types.AlignSize) * types.AlignSize
	if chunkSize == 0 {
		chunkSize = types.AlignSize
	}

	return chunkSize
}

// determineChunkSize decides the strategy (Sequential vs Parallel).
func (d *ConcurrentDownloader) determineChunkSize(fileSize int64, numConns int) int64 {
	if d.Runtime.SequentialDownload {
		// Sequential mode: Use small fixed chunks (MinChunkSize) to ensure strict ordering
		chunkSize := d.Runtime.GetMinChunkSize()
		if chunkSize <= 0 {
			chunkSize = 2 * 1024 * 1024 // Default 2MB if not configured
		}
		// Align to 4KB
		chunkSize = (chunkSize / types.AlignSize) * types.AlignSize
		if chunkSize == 0 {
			chunkSize = types.AlignSize
		}
		return chunkSize
	}

	// Parallel mode: Use large shards
	return d.calculateChunkSize(fileSize, numConns)
}

// createTasks generates initial task queue from file size and chunk size.
func createTasks(fileSize, chunkSize int64) []types.Task {
	if chunkSize <= 0 {
		return nil
	}

	// preallocate slice capacity
	count := (fileSize + chunkSize - 1) / chunkSize
	tasks := make([]types.Task, 0, int(count))

	for offset := int64(0); offset < fileSize; offset += chunkSize {
		length := chunkSize
		if offset+length > fileSize {
			length = fileSize - offset
		}
		tasks = append(tasks, types.Task{Offset: offset, Length: length})
	}
	return tasks
}

// newConcurrentClient creates an http.Client tuned for concurrent downloads.
func (d *ConcurrentDownloader) newConcurrentClients(numConns int, supportsHTTP2 bool, supportsHTTP3 bool) *clientSet {
	// Ensure we have enough connections per host
	maxConns := d.Runtime.GetMaxConnectionsPerHost()
	if numConns > maxConns {
		maxConns = numConns
	}

	// Keep proxy handling explicit to avoid surprising env interactions.
	var proxyFunc func(*http.Request) (*url.URL, error)
	if d.Runtime.ProxyURL != "" {
		if parsedURL, err := url.Parse(d.Runtime.ProxyURL); err == nil {
			proxyFunc = http.ProxyURL(parsedURL)
		} else {
			// Fallback or log error? For now fallback to environment
			utils.Debug("Invalid proxy URL %s: %v", d.Runtime.ProxyURL, err)
			proxyFunc = http.ProxyFromEnvironment
		}
	} else {
		proxyFunc = http.ProxyFromEnvironment
	}

	buildHTTPTransport := func(forceHTTP2 bool) *http.Transport {
		transport := &http.Transport{
			// Connection pooling
			MaxIdleConns:        types.DefaultMaxIdleConns,
			MaxIdleConnsPerHost: maxConns + 2, // Slightly more than max to handle bursts
			MaxConnsPerHost:     maxConns,
			Proxy:               proxyFunc,

			// Timeouts to prevent hung connections
			IdleConnTimeout:       types.DefaultIdleConnTimeout,
			TLSHandshakeTimeout:   types.DefaultTLSHandshakeTimeout,
			ResponseHeaderTimeout: types.DefaultResponseHeaderTimeout,
			ExpectContinueTimeout: types.DefaultExpectContinueTimeout,

			// Performance tuning
			DisableCompression: true, // Files are usually already compressed
			ForceAttemptHTTP2:  forceHTTP2,

			// Dial settings for TCP reliability
			DialContext: (&net.Dialer{
				Timeout:   types.DialTimeout,
				KeepAlive: types.KeepAliveDuration,
			}).DialContext,
		}

		if !forceHTTP2 {
			transport.TLSNextProto = make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)
		}

		return transport
	}

	newHTTPClient := func(transport http.RoundTripper) *http.Client {
		return &http.Client{
			Transport: transport,
			// Preserve headers on redirects for authenticated downloads.
			// These headers were explicitly provided by the caller and should remain intact.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				// Copy headers from original request to redirect request
				if len(via) > 0 {
					for key, vals := range via[0].Header {
						// Skip Range header - we set our own for parallel downloads
						if key == "Range" {
							continue
						}
						req.Header[key] = vals
					}
				}
				return nil
			},
		}
	}

	protocol := d.Runtime.GetProtocolPreference()
	if d.Runtime != nil && d.Runtime.ProxyURL != "" && supportsHTTP3 {
		utils.Debug("HTTP/3 disabled because proxy is configured")
		supportsHTTP3 = false
	}

	http1Client := protocolClient{name: types.ProtocolHTTP1, client: newHTTPClient(buildHTTPTransport(false))}
	http2Client := protocolClient{name: types.ProtocolHTTP2, client: newHTTPClient(buildHTTPTransport(true))}

	var http3Client protocolClient
	var http3Transport *http3.Transport
	if supportsHTTP3 {
		http3Transport = &http3.Transport{
			TLSClientConfig: &tls.Config{
				NextProtos: []string{"h3"},
			},
			QUICConfig: &quic.Config{
				HandshakeIdleTimeout: types.DefaultTLSHandshakeTimeout,
				MaxIdleTimeout:       types.DefaultIdleConnTimeout,
				KeepAlivePeriod:      types.KeepAliveDuration,
			},
		}
		http3Client = protocolClient{name: types.ProtocolHTTP3, client: newHTTPClient(http3Transport)}
	}

	formatNames := func(primary protocolClient, fallbacks []protocolClient) string {
		if len(fallbacks) == 0 {
			return primary.name
		}
		names := make([]string, 0, 1+len(fallbacks))
		names = append(names, primary.name)
		for _, fallback := range fallbacks {
			names = append(names, fallback.name)
		}
		return strings.Join(names, " -> ")
	}

	makeSet := func(primary protocolClient, fallbacks ...protocolClient) *clientSet {
		set := &clientSet{primary: primary, fallbacks: fallbacks}
		if http3Transport != nil {
			if primary.name == types.ProtocolHTTP3 {
				set.http3Transport = http3Transport
			} else {
				for _, fallback := range fallbacks {
					if fallback.name == types.ProtocolHTTP3 {
						set.http3Transport = http3Transport
						break
					}
				}
			}
		}
		utils.Debug("Transport selection: pref=%s supports[h2=%t h3=%t] chain=%s", protocol, supportsHTTP2, supportsHTTP3, formatNames(primary, fallbacks))
		return set
	}

	switch protocol {
	case types.ProtocolHTTP1:
		return makeSet(http1Client)
	case types.ProtocolHTTP2:
		if supportsHTTP2 {
			return makeSet(http2Client, http1Client)
		}
		return makeSet(http1Client)
	case types.ProtocolHTTP3:
		if supportsHTTP3 {
			return makeSet(http3Client, http1Client)
		}
		return makeSet(http1Client)
	default:
		if supportsHTTP3 {
			if supportsHTTP2 {
				return makeSet(http3Client, http2Client, http1Client)
			}
			return makeSet(http3Client, http1Client)
		}
		if supportsHTTP2 {
			return makeSet(http2Client, http1Client)
		}
		return makeSet(http1Client)
	}
}

// Download downloads a file using multiple concurrent connections.
// Uses pre-probed metadata so we can preallocate and size the task queue.
func (d *ConcurrentDownloader) Download(ctx context.Context, rawurl string, candidateMirrors []string, activeMirrors []string, destPath string, fileSize int64, supportsHTTP2 bool, supportsHTTP3 bool) error {
	utils.Debug("ConcurrentDownloader.Download: %s -> %s (size: %d, mirrors: %d)", rawurl, destPath, fileSize, len(activeMirrors))

	// Store URL and path for pause/resume (final path without .GoFetch)
	d.URL = rawurl
	d.DestPath = destPath

	// Initialize mirror status in state
	if d.State != nil {
		var statuses []types.MirrorStatus
		// Add primary
		statuses = append(statuses, types.MirrorStatus{URL: rawurl, Active: true})

		// Add active mirrors (marked active)
		activeMap := make(map[string]bool)
		for _, m := range activeMirrors {
			activeMap[m] = true
			if m != rawurl {
				statuses = append(statuses, types.MirrorStatus{URL: m, Active: true})
			}
		}

		// Add inactive/failed mirrors (from candidate list that aren't active)
		for _, m := range candidateMirrors {
			if !activeMap[m] && m != rawurl {
				// Mark as Error since they failed probing (passed as candidates but not active)
				statuses = append(statuses, types.MirrorStatus{URL: m, Active: false, Error: true})
			}
		}

		d.State.SetMirrors(statuses)
	}

	// Working file has .GoFetch suffix until download completes.
	workingPath := destPath + types.IncompleteSuffix

	// Create cancellable context for pause support
	downloadCtx, cancel := context.WithCancel(ctx)

	// Helper synchronization for balancer/monitor goroutines.
	var wgHelpers sync.WaitGroup
	// Ensure we wait for helpers to finish; run wait AFTER cancel (LIFO: cancel runs first)
	defer wgHelpers.Wait()
	defer cancel()

	if d.State != nil {
		d.State.SetCancelFunc(cancel)
	}

	// Determine connections and chunk size.
	numConns := d.getInitialConnections(fileSize)
	chunkSize := d.determineChunkSize(fileSize, numConns)

	// Create tuned HTTP clients for concurrent downloads
	clients := d.newConcurrentClients(numConns, supportsHTTP2, supportsHTTP3)
	defer clients.Close()

	// Initialize chunk visualization
	if d.State != nil {
		d.State.InitBitmap(fileSize, chunkSize)
	}

	// Create and preallocate output file with .GoFetch suffix.
	outFile, err := os.OpenFile(workingPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer func() {
		if err := outFile.Close(); err != nil {
			utils.Debug("Error closing file: %v", err)
		}
	}()

	tasks := createTasks(fileSize, chunkSize)
	// Check for saved state BEFORE truncating (resume case).
	savedState, err := state.LoadState(rawurl, destPath)
	isResume := err == nil && savedState != nil && len(savedState.Tasks) > 0

	if isResume {
		// Resume: use saved tasks and restore downloaded counter.
		tasks = savedState.Tasks
		if d.State != nil {
			d.State.Downloaded.Store(savedState.Downloaded)
			d.State.VerifiedProgress.Store(savedState.Downloaded)
			// Restore elapsed time from previous sessions
			d.State.SetSavedElapsed(time.Duration(savedState.Elapsed))
			// Fix speed spike: sync session start so we don't count previous bytes as new speed
			d.State.SyncSessionStart()

			// Restore chunk bitmap to keep UI in sync after resume.
			if len(savedState.ChunkBitmap) > 0 && savedState.ActualChunkSize > 0 {
				d.State.RestoreBitmap(savedState.ChunkBitmap, savedState.ActualChunkSize)

				// Reconstruct internal progress from remaining tasks to ensure partial chunks are handled correctly
				d.State.RecalculateProgress(savedState.Tasks)
				// Keep counters aligned after reconstruction to avoid session speed spikes.
				d.State.Downloaded.Store(d.State.VerifiedProgress.Load())
				d.State.SyncSessionStart()

				utils.Debug("Restored chunk map: size %d", savedState.ActualChunkSize)
			}
		}
		utils.Debug("Resuming from saved state: %d tasks, %d bytes downloaded", len(tasks), savedState.Downloaded)
	} else {
		// Fresh download: preallocate file and create new tasks.
		if err := outFile.Truncate(fileSize); err != nil {
			return fmt.Errorf("failed to preallocate file: %w", err)
		}
		// Robustness: ensure state counter starts at 0 for fresh download
		if d.State != nil {
			d.State.Downloaded.Store(0)
			d.State.SyncSessionStart()
		}
	}
	queue := NewTaskQueue()
	queue.PushMultiple(tasks)

	// Start time for stats
	startTime := time.Now()

	// Start balancer goroutine for dynamic chunk splitting.
	balancerCtx, cancelBalancer := context.WithCancel(downloadCtx)
	defer cancelBalancer()

	wgHelpers.Add(1)
	go func() {
		defer wgHelpers.Done()
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-balancerCtx.Done():
				return
			case <-ticker.C:
				// Aggressively fill idle workers
				// Continue splitting/stealing as long as we have idle workers and are making progress
				for queue.IdleWorkers() > 0 {
					didWork := false
					if queue.Len() == 0 {
						// Try to steal from an active worker
						if d.StealWork(queue) {
							didWork = true
						}
					}

					// If stealing failed (chunks too small), try hedged request:
					// Duplicate a task so an idle worker races on a fresh connection
					if !didWork && queue.Len() == 0 {
						if d.HedgeWork(queue) {
							didWork = true
						}
					}

					// If we couldn't split, steal, or hedge anything, stop trying for this tick
					if !didWork {
						break
					}
				}
			}
		}
	}()

	// Monitor for completion.
	wgHelpers.Add(1)
	go func() {
		defer wgHelpers.Done()
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				queue.Close()
				return
			case <-balancerCtx.Done():
				queue.Close()
				return
			case <-ticker.C:
				// Ensure queue is empty (no pending retries) before considering byte count.
				// This avoids early exit when overlaps inflate counters.
				if queue.Len() == 0 && (int(queue.IdleWorkers()) == numConns || (d.State != nil && d.State.Downloaded.Load() >= fileSize)) {
					queue.Close()
					return
				}
			}
		}
	}()

	wgHelpers.Add(1)
	go func() {
		defer wgHelpers.Done()
		ticker := time.NewTicker(types.HealthCheckInterval) // Fixed: using types constant
		defer ticker.Stop()

		for {
			select {
			case <-balancerCtx.Done():
				return
			case <-ticker.C:
				d.checkWorkerHealth()
			}
		}
	}()

	// Start workers.
	var wg sync.WaitGroup
	workerErrors := make(chan error, numConns)

	// Combine primary + secondary for workers
	// We want to ensure the primary is included if it was valid (it should be, otherwise TUIDownload would have failed)
	var workerMirrors []string

	// Add primary if compatible (check active map or assume yes since we are here)
	// TUIDownload checks primary support before calling us.
	workerMirrors = append(workerMirrors, rawurl)

	// Add other valid mirrors
	for _, v := range activeMirrors {
		if v != rawurl {
			workerMirrors = append(workerMirrors, v)
		}
	}

	// Double check we have at least one mirror
	if len(workerMirrors) == 0 {
		// Should have been caught by early check but safe fallback
		workerMirrors = []string{rawurl}
	}

	for i := 0; i < numConns; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			err := d.worker(downloadCtx, workerID, workerMirrors, outFile, queue, fileSize, startTime, clients)
			if err != nil && err != context.Canceled {
				workerErrors <- err
			}
		}(i)
	}

	// Wait for all workers to complete
	go func() {
		wg.Wait()
		close(workerErrors)
		queue.Close()
	}()

	// Check for errors or pause
	var downloadErr error
	for err := range workerErrors {
		if err != nil {
			downloadErr = err
		}
	}

	// Handle pause: state saved.
	if d.State != nil && d.State.IsPaused() {
		// 1. Collect active tasks as remaining work FIRST.
		var activeRemaining []types.Task
		d.activeMu.Lock()
		for _, active := range d.activeTasks {
			if remaining := active.RemainingTask(); remaining != nil {
				activeRemaining = append(activeRemaining, *remaining)
			}
		}
		d.activeMu.Unlock()

		// 2. Collect remaining tasks from queue.
		remainingTasks := queue.DrainRemaining()
		remainingTasks = append(remainingTasks, activeRemaining...)

		// Calculate Downloaded from remaining tasks (ensures consistency).
		var remainingBytes int64
		for _, task := range remainingTasks {
			remainingBytes += task.Length
		}
		computedDownloaded := fileSize - remainingBytes

		// Calculate total elapsed time
		var totalElapsed time.Duration
		var chunkBitmap []byte
		var actualChunkSize int64

		if d.State != nil {
			totalElapsed = d.State.GetSavedElapsed() + time.Since(startTime)
			// Get persisted bitmap data
			bitmap, _, _, chunkSize, _ := d.State.GetBitmap()
			chunkBitmap = bitmap
			actualChunkSize = chunkSize
			// Keep in-memory state aligned with the persisted snapshot.
			d.State.FinalizePause(computedDownloaded, totalElapsed)
		} else {
			totalElapsed = time.Since(startTime)
		}

		// Save state for resume (use computed value for consistency).
		s := &types.DownloadState{
			URL:             d.URL,
			ID:              d.ID,
			DestPath:        destPath,
			TotalSize:       fileSize,
			Downloaded:      computedDownloaded,
			Tasks:           remainingTasks,
			Filename:        filepath.Base(destPath),
			Elapsed:         totalElapsed.Nanoseconds(),
			Mirrors:         candidateMirrors,
			ChunkBitmap:     chunkBitmap,
			ActualChunkSize: actualChunkSize,
		}
		if err := state.SaveState(d.URL, destPath, s); err != nil {
			utils.Debug("Failed to save pause state: %v", err)
		}

		utils.Debug("Download paused, state saved (Downloaded=%d, RemainingTasks=%d, RemainingBytes=%d)",
			computedDownloaded, len(remainingTasks), remainingBytes)
		return types.ErrPaused // Signal valid pause to caller
	}

	// Handle cancel: context was cancelled but not via Pause().
	// Propagate cancellation so callers don't treat this as a successful completion.
	if downloadCtx.Err() == context.Canceled {
		return context.Canceled
	}

	if downloadErr != nil {
		return downloadErr
	}

	// Final sync
	if err := outFile.Sync(); err != nil {
		return fmt.Errorf("failed to sync file: %w", err)
	}

	// Close file before renaming
	_ = outFile.Close()

	// Rename from .GoFetch to final destination.
	if err := os.Rename(workingPath, destPath); err != nil {
		// Check for race condition: did someone else already rename it?
		if os.IsNotExist(err) {
			if info, statErr := os.Stat(destPath); statErr == nil && info.Size() == fileSize {
				utils.Debug("Race condition detected: File already exists and has correct size. Treating as success.")
				// Clean up state just in case, though usually done by caller
				_ = state.DeleteState(d.ID, d.URL, destPath)
				return nil
			}
		}
		return fmt.Errorf("failed to rename completed file: %w", err)
	}

	// Delete state file on successful completion.
	_ = state.DeleteState(d.ID, d.URL, destPath)

	// Note: Download completion notifications are handled by the TUI via DownloadCompleteMsg
	return nil
}
