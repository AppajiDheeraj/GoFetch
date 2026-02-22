package cli

import (
	"concurrent_downloader/internal/clipboard"
	"concurrent_downloader/internal/config"
	"concurrent_downloader/internal/core"
	"concurrent_downloader/internal/download"
	"concurrent_downloader/internal/download/types"
	"concurrent_downloader/internal/events"
	"concurrent_downloader/internal/state"
	"concurrent_downloader/internal/utils"
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// Version information - set via ldflags during build.
var (
	Version   = "dev"
	BuildTime = "unknown"
)

// activeDownloads tracks the number of currently running downloads in headless mode
var activeDownloads int32

// Command line flags
var verbose bool

// Globals for Unified Backend
var (
	GlobalPool       *download.WorkerPool
	GlobalProgressCh chan any
	GlobalService    core.DownloadService
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:     "GoFetch [url]...",
	Short:   "An open-source download manager written in Go",
	Long:    `GoFetch is a blazing fast, open-source terminal (TUI) download manager built in Go.`,
	Version: Version,
	Args:    cobra.ArbitraryArgs,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		utils.SetVerbose(verbose)

		// Initialize Global Progress Channel.
		GlobalProgressCh = make(chan any, 100)

		// Initialize Global Worker Pool using settings for concurrency limits.
		settings, err := config.LoadSettings()
		if err != nil {
			settings = config.DefaultSettings()
		}
		GlobalPool = download.NewWorkerPool(GlobalProgressCh, settings.Network.MaxConcurrentDownloads)
	},
	Run: func(cmd *cobra.Command, args []string) {
		initializeGlobalState()

		// Check for clipboard flag to append a URL without manual paste.
		clipboardFlag, _ := cmd.Flags().GetBool("clipboard")
		if clipboardFlag {
			url, err := clipboard.ReadURL()
			if err != nil {
				if err == clipboard.ErrInvalidURL {
					fmt.Fprintln(os.Stderr, "Error: Clipboard does not contain a valid URL")
				} else {
					fmt.Fprintf(os.Stderr, "Error reading from clipboard: %v\n", err)
				}
				os.Exit(1)
			}
			args = append(args, url)
			fmt.Printf("URL from clipboard: %s\n", url)
		}

		// Validate integrity of paused downloads before resuming.
		// Removes entries whose .GoFetch files are missing or tampered with.
		if removed, err := state.ValidateIntegrity(); err != nil {
			utils.Debug("Integrity check failed: %v", err)
		} else if removed > 0 {
			utils.Debug("Integrity check: removed %d corrupted/orphaned downloads", removed)
		}

		// Attempt to acquire lock to enforce single-instance server semantics.
		isMaster, err := AcquireLock()
		if err != nil {
			fmt.Printf("Error acquiring lock: %v\n", err)
			os.Exit(1)
		}

		if !isMaster {
			fmt.Fprintln(os.Stderr, "Error: GoFetch is already running.")
			fmt.Fprintln(os.Stderr, "Use 'GoFetch add <url>' to add a download to the active instance.")
			os.Exit(1)
		}
		defer func() {
			if err := ReleaseLock(); err != nil {
				utils.Debug("Error releasing lock: %v", err)
			}
		}()

		// Initialize Service.
		GlobalService = core.NewLocalDownloadServiceWithInput(GlobalPool, GlobalProgressCh)

		portFlag, _ := cmd.Flags().GetInt("port")
		batchFile, _ := cmd.Flags().GetString("batch")
		outputDir, _ := cmd.Flags().GetString("output")
		filename, _ := cmd.Flags().GetString("filename")
		forceSingle, _ := cmd.Flags().GetBool("force-single")
		chunkCount, _ := cmd.Flags().GetInt("chunks")
		noResume, _ := cmd.Flags().GetBool("no-resume")
		exitWhenDone, _ := cmd.Flags().GetBool("exit-when-done")

		port, listener, err := bindServerListener(portFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}

		// Save port for browser extension AND CLI discovery.
		saveActivePort(port)
		defer removeActivePort()

		// Start HTTP server in background (reuse the listener).
		go startHTTPServer(listener, port, outputDir, GlobalService)

		// Queue initial downloads if any.
		go func() {
			var urls []string
			urls = append(urls, args...)

			if batchFile != "" {
				fileUrls, err := readURLsFromFile(batchFile)
				if err != nil {
					fmt.Fprintf(os.Stderr, "Error reading batch file: %v\n", err)
				} else {
					urls = append(urls, fileUrls...)
				}
			}

			if len(urls) > 0 {
				if filename != "" && len(urls) > 1 {
					fmt.Fprintln(os.Stderr, "Error: --filename can only be used with a single URL")
					return
				}
				if forceSingle && chunkCount > 0 {
					fmt.Fprintln(os.Stderr, "Error: --chunks cannot be used with --force-single")
					return
				}
				if chunkCount < 0 {
					fmt.Fprintln(os.Stderr, "Error: --chunks must be a positive number")
					return
				}
				processDownloads(urls, outputDir, filename, forceSingle, chunkCount, 0) // 0 port = internal direct add
			}
		}()

		// Start CLI mode (headless).
		startCLI(exitWhenDone, noResume)
	},
}

// startCLI runs the headless loop and handles shutdown signals.
func startCLI(exitWhenDone bool, noResume bool) {
	// Start headless event consumer for CLI output.
	StartHeadlessConsumer()

	// Auto-resume paused downloads if enabled.
	if !noResume {
		resumePausedDownloads()
	}

	fmt.Println("GoFetch is running. Press Ctrl+C to exit.")

	// Exit-when-done checker for CLI mode.
	if exitWhenDone {
		go func() {
			// Wait a bit for initial downloads to be queued
			time.Sleep(3 * time.Second)
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				if GlobalPool != nil && GlobalPool.ActiveCount() == 0 {
					fmt.Println("All downloads completed. Exiting...")
					_ = executeGlobalShutdown("cli: all downloads done")
					os.Exit(0)
				}
			}
		}()
	}

	// Signal handler for graceful shutdown.
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigChan)

	// Wait for signal
	sig := <-sigChan
	fmt.Printf("\nReceived signal: %s. Shutting down...\n", sig)
	_ = executeGlobalShutdown(fmt.Sprintf("cli signal: %s", sig))
}

func getServerBindHost() string {
	return "0.0.0.0"
}

func StartHeadlessConsumer() {
	// Headless consumer keeps CLI output responsive without blocking downloads.
	go func() {
		if GlobalService == nil {
			return
		}
		progressState := make(map[string]*cliProgressState)
		lastInlineID := ""
		stream, cleanup, err := GlobalService.StreamEvents(context.Background())
		if err != nil {
			utils.Debug("Failed to start event stream: %v", err)
			return
		}
		defer cleanup()

		for msg := range stream {
			switch m := msg.(type) {
			case events.DownloadStartedMsg:
				finalizeInline(&lastInlineID)
				id := m.DownloadID
				if len(id) > 8 {
					id = id[:8]
				}
				state := getOrCreateProgressState(progressState, m.DownloadID)
				state.filename = m.Filename
				state.total = m.Total
				state.started = true
				fmt.Printf("Started: %s [%s]\n", m.Filename, id)
			case events.DownloadCompleteMsg:
				finalizeInline(&lastInlineID)
				atomic.AddInt32(&activeDownloads, -1)
				id := m.DownloadID
				if len(id) > 8 {
					id = id[:8]
				}
				delete(progressState, m.DownloadID)
				fmt.Printf("Completed: %s [%s] (in %s)\n", m.Filename, id, m.Elapsed)
			case events.DownloadErrorMsg:
				finalizeInline(&lastInlineID)
				atomic.AddInt32(&activeDownloads, -1)
				id := m.DownloadID
				if len(id) > 8 {
					id = id[:8]
				}
				delete(progressState, m.DownloadID)
				fmt.Printf("Error: %s [%s]: %v\n", m.Filename, id, m.Err)
			case events.DownloadQueuedMsg:
				finalizeInline(&lastInlineID)
				id := m.DownloadID
				if len(id) > 8 {
					id = id[:8]
				}
				state := getOrCreateProgressState(progressState, m.DownloadID)
				if state.filename == "" {
					state.filename = m.Filename
				}
				fmt.Printf("Queued: %s [%s]\n", m.Filename, id)
			case events.DownloadPausedMsg:
				finalizeInline(&lastInlineID)
				id := m.DownloadID
				if len(id) > 8 {
					id = id[:8]
				}
				fmt.Printf("Paused: %s [%s]\n", m.Filename, id)
			case events.DownloadResumedMsg:
				finalizeInline(&lastInlineID)
				id := m.DownloadID
				if len(id) > 8 {
					id = id[:8]
				}
				fmt.Printf("Resumed: %s [%s]\n", m.Filename, id)
			case events.DownloadRemovedMsg:
				finalizeInline(&lastInlineID)
				id := m.DownloadID
				if len(id) > 8 {
					id = id[:8]
				}
				delete(progressState, m.DownloadID)
				fmt.Printf("Removed: %s [%s]\n", m.Filename, id)
			case events.ProgressMsg:
				renderProgressLine(m, progressState, &lastInlineID)
			case events.BatchProgressMsg:
				for _, p := range m {
					renderProgressLine(p, progressState, &lastInlineID)
				}
			}
		}
	}()
}

type cliProgressState struct {
	filename    string
	total       int64
	lastPercent int
	lastPrint   time.Time
	started     bool
}

func getOrCreateProgressState(progressState map[string]*cliProgressState, id string) *cliProgressState {
	state := progressState[id]
	if state == nil {
		state = &cliProgressState{}
		progressState[id] = state
	}
	return state
}

func renderProgressLine(m events.ProgressMsg, progressState map[string]*cliProgressState, lastInlineID *string) {
	state := getOrCreateProgressState(progressState, m.DownloadID)
	if !state.started {
		return
	}
	if state.total <= 0 && m.Total > 0 {
		state.total = m.Total
	}
	if state.filename == "" {
		state.filename = shortID(m.DownloadID)
	}
	percent := 0
	if state.total > 0 {
		percent = int(float64(m.Downloaded) * 100 / float64(state.total))
	}
	now := time.Now()
	if percent == state.lastPercent && now.Sub(state.lastPrint) < 750*time.Millisecond {
		return
	}
	state.lastPercent = percent
	state.lastPrint = now

	speed := m.Speed
	eta := formatETA(state.total, m.Downloaded, speed)
	if state.total > 0 && percent >= 100 {
		avgSpeed := 0.0
		if m.Elapsed > 0 {
			avgSpeed = float64(m.Downloaded) / m.Elapsed.Seconds()
		}
		if avgSpeed > 0 {
			speed = avgSpeed
		}
		eta = "00:00"
	}

	line := formatProgressLine(state.filename, m.DownloadID, m.Downloaded, state.total, speed, eta)
	inline := atomic.LoadInt32(&activeDownloads) <= 1
	if inline {
		*lastInlineID = m.DownloadID
		fmt.Printf("\r%s", line)
		return
	}
	finalizeInline(lastInlineID)
	fmt.Println(line)
}

func finalizeInline(lastInlineID *string) {
	if lastInlineID == nil || *lastInlineID == "" {
		return
	}
	fmt.Print("\n")
	*lastInlineID = ""
}

func formatProgressLine(filename string, id string, downloaded int64, total int64, speed float64, etaLabel string) string {
	short := shortID(id)
	speedLabel := formatSpeed(speed)
	if total <= 0 {
		return fmt.Sprintf("%s [%s] %s %s", filename, short, utils.ConvertBytesToHumanReadable(downloaded), speedLabel)
	}
	percent := float64(downloaded) * 100 / float64(total)
	bar := renderProgressBar(percent, 24)
	return fmt.Sprintf("%s [%s] |%s| %5.1f%% %s ETA %s", filename, short, bar, percent, speedLabel, etaLabel)
}

func renderProgressBar(percent float64, width int) string {
	if width <= 0 {
		return ""
	}
	if percent < 0 {
		percent = 0
	}
	if percent > 100 {
		percent = 100
	}
	fill := int(percent * float64(width) / 100)
	if fill < 0 {
		fill = 0
	}
	if fill > width {
		fill = width
	}
	return strings.Repeat("#", fill) + strings.Repeat("-", width-fill)
}

func formatSpeed(speed float64) string {
	if speed <= 0 {
		return "0 B/s"
	}
	return fmt.Sprintf("%s/s", utils.ConvertBytesToHumanReadable(int64(speed)))
}

func formatETA(total int64, downloaded int64, speed float64) string {
	if total <= 0 || speed <= 0 || downloaded >= total {
		return "--:--"
	}
	remaining := float64(total-downloaded) / speed
	if remaining < 0 {
		remaining = 0
	}
	seconds := int64(remaining + 0.5)
	if seconds < 60 {
		return fmt.Sprintf("00:%02d", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	secs := seconds % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, secs)
}

func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// findAvailablePort tries ports starting from 'start' until one is available
// findAvailablePort scans for a free TCP port starting at the given base.
func findAvailablePort(start int) (int, net.Listener) {
	bindHost := getServerBindHost()
	for port := start; port < start+100; port++ {
		ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", bindHost, port))
		if err == nil {
			return port, ln
		}
	}
	return 0, nil
}

// bindServerListener resolves the port selection policy and returns a listener.
func bindServerListener(portFlag int) (int, net.Listener, error) {
	bindHost := getServerBindHost()
	if portFlag > 0 {
		ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", bindHost, portFlag))
		if err != nil {
			return 0, nil, fmt.Errorf("could not bind to port %d: %w", portFlag, err)
		}
		return portFlag, ln, nil
	}
	port, ln := findAvailablePort(1700)
	if ln == nil {
		return 0, nil, fmt.Errorf("could not find available port")
	}
	return port, ln, nil
}

// saveActivePort writes the active port to ~/.GoFetch/port for extension discovery.
func saveActivePort(port int) {
	portFile := filepath.Join(config.GetRuntimeDir(), "port")
	if err := os.WriteFile(portFile, []byte(fmt.Sprintf("%d", port)), 0o644); err != nil {
		utils.Debug("Error writing port file: %v", err)
	}
	utils.Debug("HTTP server listening on port %d", port)
}

// removeActivePort cleans up the port file on exit.
func removeActivePort() {
	portFile := filepath.Join(config.GetRuntimeDir(), "port")
	if err := os.Remove(portFile); err != nil && !os.IsNotExist(err) {
		utils.Debug("Error removing port file: %v", err)
	}
}

// startHTTPServer starts the local control plane used by the browser extension and CLI.
func startHTTPServer(ln net.Listener, port int, defaultOutputDir string, service core.DownloadService) {
	authToken := ensureAuthToken()

	mux := http.NewServeMux()

	// Health check endpoint (Public)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"port":   port,
		}); err != nil {
			utils.Debug("Failed to encode response: %v", err)
		}
	})

	// SSE Events Endpoint (Protected).
	mux.HandleFunc("/events", func(w http.ResponseWriter, r *http.Request) {
		// Set headers for SSE.
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		// Get event stream.
		stream, cleanup, err := service.StreamEvents(r.Context())
		if err != nil {
			http.Error(w, "Failed to subscribe to events", http.StatusInternalServerError)
			return
		}
		defer cleanup()

		// Flush headers immediately so the client knows the stream is live.
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
			return
		}
		flusher.Flush()

		// Send events.
		// Create a closer notifier.
		done := r.Context().Done()

		for {
			select {
			case <-done:
				return
			case msg, ok := <-stream:
				if !ok {
					return
				}

				// Encode message to JSON.
				data, err := json.Marshal(msg)
				if err != nil {
					utils.Debug("Error marshaling event: %v", err)
					continue
				}

				// Determine event type name based on struct.
				eventType := "unknown"
				switch msg := msg.(type) {
				case events.DownloadStartedMsg:
					eventType = "started"
				case events.DownloadCompleteMsg:
					eventType = "complete"
				case events.DownloadErrorMsg:
					eventType = "error"
				case events.ProgressMsg:
					eventType = "progress"
				case events.DownloadPausedMsg:
					eventType = "paused"
				case events.DownloadResumedMsg:
					eventType = "resumed"
				case events.DownloadQueuedMsg:
					eventType = "queued"
				case events.DownloadRemovedMsg:
					eventType = "removed"
				case events.DownloadRequestMsg:
					eventType = "request"
				case events.BatchProgressMsg:
					// Unroll batch and send individual progress events
					for _, p := range msg {
						data, _ := json.Marshal(p)
						_, _ = fmt.Fprintf(w, "event: progress\n")
						_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
					}
					flusher.Flush()
					continue // Skip default send
				}

				// SSE Format:
				// event: <type>
				// data: <json>
				// \n
				_, _ = fmt.Fprintf(w, "event: %s\n", eventType)
				_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	})

	// Download endpoint (Protected).
	mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		handleDownload(w, r, defaultOutputDir, service)
	})

	// Pause endpoint (Protected).
	mux.HandleFunc("/pause", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id parameter", http.StatusBadRequest)
			return
		}

		if err := service.Pause(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "paused", "id": id}); err != nil {
			utils.Debug("Failed to encode response: %v", err)
		}
	})

	// Resume endpoint (Protected).
	mux.HandleFunc("/resume", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id parameter", http.StatusBadRequest)
			return
		}

		if err := service.Resume(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "resumed", "id": id}); err != nil {
			utils.Debug("Failed to encode response: %v", err)
		}
	})

	// Delete endpoint (Protected).
	mux.HandleFunc("/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete && r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id parameter", http.StatusBadRequest)
			return
		}

		if err := service.Delete(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "id": id}); err != nil {
			utils.Debug("Failed to encode response: %v", err)
		}
	})

	// List endpoint (Protected).
	mux.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		statuses, err := service.List()
		if err != nil {
			http.Error(w, "Failed to list downloads: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(statuses); err != nil {
			utils.Debug("Failed to encode response: %v", err)
		}
	})

	// History endpoint (Protected).
	mux.HandleFunc("/history", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		history, err := service.History()
		if err != nil {
			http.Error(w, "Failed to retrieve history: "+err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(history); err != nil {
			utils.Debug("Failed to encode response: %v", err)
		}
	})

	// Wrap mux with Auth and CORS (CORS outermost to ensure 401/403 include headers).
	handler := corsMiddleware(authMiddleware(authToken, mux))

	server := &http.Server{Handler: handler}
	if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
		utils.Debug("HTTP server error: %v", err)
	}
}

// corsMiddleware keeps extension and local tools unblocked across origins.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set CORS headers
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS, PUT, PATCH")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With, Access-Control-Allow-Private-Network")
		w.Header().Set("Access-Control-Allow-Private-Network", "true")

		// Handle preflight requests.
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// authMiddleware protects control endpoints with a shared bearer token.
func authMiddleware(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow health check without auth.
		if r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		// Allow OPTIONS for CORS preflight.
		if r.Method == "OPTIONS" {
			next.ServeHTTP(w, r)
			return
		}

		// Check for Authorization header.
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" {
			if strings.HasPrefix(authHeader, "Bearer ") {
				providedToken := strings.TrimPrefix(authHeader, "Bearer ")
				if len(providedToken) == len(token) && subtle.ConstantTimeCompare([]byte(providedToken), []byte(token)) == 1 {
					next.ServeHTTP(w, r)
					return
				}
			}
		}

		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

// ensureAuthToken loads or generates the daemon auth token.
func ensureAuthToken() string {
	tokenFile := filepath.Join(config.GetStateDir(), "token")
	data, err := os.ReadFile(tokenFile)
	if err == nil {
		return strings.TrimSpace(string(data))
	}

	// Generate new token
	token := uuid.New().String()

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(tokenFile), 0755); err != nil {
		utils.Debug("Failed to create token directory: %v", err)
	}
	if err := os.WriteFile(tokenFile, []byte(token), 0600); err != nil {
		utils.Debug("Failed to write token file: %v", err)
	}
	return token
}

type DownloadRequest struct {
	URL                  string            `json:"url"`
	Filename             string            `json:"filename,omitempty"`
	Path                 string            `json:"path,omitempty"`
	RelativeToDefaultDir bool              `json:"relative_to_default_dir,omitempty"`
	Mirrors              []string          `json:"mirrors,omitempty"`
	SkipApproval         bool              `json:"skip_approval,omitempty"` // Extension validated request, skip TUI prompt
	Headers              map[string]string `json:"headers,omitempty"`       // Custom HTTP headers from browser (cookies, auth, etc.)
	ForceSingle          bool              `json:"force_single,omitempty"`
	ChunkCount           int               `json:"chunk_count,omitempty"`
}

// handleDownload implements both GET status lookup and POST enqueue.
func handleDownload(w http.ResponseWriter, r *http.Request, defaultOutputDir string, service core.DownloadService) {
	// GET request to query status
	if r.Method == http.MethodGet {
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id parameter", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if service == nil {
			http.Error(w, "Service unavailable", http.StatusInternalServerError)
			return
		}

		status, err := service.GetStatus(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		if err := json.NewEncoder(w).Encode(status); err != nil {
			utils.Debug("Failed to encode response: %v", err)
		}
		return
	}

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Load settings once for use throughout the function.
	settings, err := config.LoadSettings()
	if err != nil {
		// Fallback to defaults if loading fails (though LoadSettings handles missing file)
		settings = config.DefaultSettings()
	}

	var req DownloadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer func() {
		if err := r.Body.Close(); err != nil {
			utils.Debug("Error closing body: %v", err)
		}
	}()

	if req.URL == "" {
		http.Error(w, "URL is required", http.StatusBadRequest)
		return
	}

	if req.ChunkCount < 0 {
		http.Error(w, "chunk_count must be a positive number", http.StatusBadRequest)
		return
	}
	if req.ChunkCount > 0 && req.ForceSingle {
		http.Error(w, "chunk_count cannot be used with force_single", http.StatusBadRequest)
		return
	}

	// Prevent directory traversal through API payloads.
	if strings.Contains(req.Path, "..") || strings.Contains(req.Filename, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	if strings.Contains(req.Filename, "/") || strings.Contains(req.Filename, "\\") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	utils.Debug("Received download request: URL=%s, Path=%s", req.URL, req.Path)

	if service == nil {
		http.Error(w, "Service unavailable", http.StatusInternalServerError)
		return
	}

	// Prepare output path with consistent absolute form for resume stability.
	outPath := req.Path
	if req.RelativeToDefaultDir && req.Path != "" {
		// Resolve relative to default download directory
		baseDir := settings.General.DefaultDownloadDir
		if baseDir == "" {
			baseDir = defaultOutputDir
		}
		if baseDir == "" {
			baseDir = "."
		}
		outPath = filepath.Join(baseDir, req.Path)
		if err := os.MkdirAll(outPath, 0o755); err != nil {
			http.Error(w, "Failed to create directory: "+err.Error(), http.StatusInternalServerError)
			return
		}

	} else if outPath == "" {
		if defaultOutputDir != "" {
			outPath = defaultOutputDir
			if err := os.MkdirAll(outPath, 0o755); err != nil {
				http.Error(w, "Failed to create output directory: "+err.Error(), http.StatusInternalServerError)
				return
			}
		} else {
			if settings.General.DefaultDownloadDir != "" {
				outPath = settings.General.DefaultDownloadDir
				if err := os.MkdirAll(outPath, 0o755); err != nil {
					http.Error(w, "Failed to create output directory: "+err.Error(), http.StatusInternalServerError)
					return
				}
			} else {
				outPath = "."
			}
		}
	}

	// Enforce absolute path to ensure resume works even if CWD changes.
	outPath = utils.EnsureAbsPath(outPath)

	// Check settings for extension prompt and duplicates.
	// Distinguish ACTIVE (corruption risk) and COMPLETED (overwrite safe).
	isDuplicate := false
	isActive := false

	urlForAdd := req.URL
	mirrorsForAdd := req.Mirrors
	if len(mirrorsForAdd) == 0 && strings.Contains(req.URL, ",") {
		urlForAdd, mirrorsForAdd = ParseURLArg(req.URL)
	}

	if GlobalPool.HasDownload(urlForAdd) {
		isDuplicate = true
		// Check if specifically active\
		allActive := GlobalPool.GetAll()
		for _, c := range allActive {
			if c.URL == urlForAdd {
				if c.State != nil && !c.State.Done.Load() {
					isActive = true
				}
				break
			}
		}
	}

	utils.Debug("Download request: URL=%s, SkipApproval=%v, isDuplicate=%v, isActive=%v", urlForAdd, req.SkipApproval, isDuplicate, isActive)

	// Extension vetting shortcut: when SkipApproval is true, trust the extension.
	// The backend will auto-rename duplicate files, so no need to reject.
	if req.SkipApproval {
		// Trust extension -> Skip all prompting logic, proceed to download
		utils.Debug("Extension request: skipping all prompts, proceeding with download")
	} else {
		// In CLI mode, respect settings but don't prompt interactively
		// If WarnOnDuplicate is enabled AND it's an active download, reject it
		if settings.General.WarnOnDuplicate && isActive {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			if err := json.NewEncoder(w).Encode(map[string]string{
				"status":  "error",
				"message": "Download rejected: Active duplicate download detected",
			}); err != nil {
				utils.Debug("Failed to encode response: %v", err)
			}
			return
		}
	}

	// Add via service.
	var opts *types.AddOptions
	if req.ForceSingle || req.ChunkCount > 0 {
		opts = &types.AddOptions{ForceSingle: req.ForceSingle, ChunkCount: req.ChunkCount}
	}
	newID, err := service.Add(urlForAdd, outPath, req.Filename, mirrorsForAdd, req.Headers, opts)
	if err != nil {
		http.Error(w, "Failed to add download: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Increment active downloads counter.
	atomic.AddInt32(&activeDownloads, 1)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status":  "queued",
		"message": "Download queued successfully",
		"id":      newID,
	}); err != nil {
		utils.Debug("Failed to encode response: %v", err)
	}
}

// processDownloads handles the logic of adding downloads either to local pool or remote server.
// Returns the number of successfully added downloads.
func processDownloads(urls []string, outputDir string, filename string, forceSingle bool, chunkCount int, port int) int {
	successCount := 0

	// If port > 0, send to a remote server.
	if port > 0 {
		for _, arg := range urls {
			url, mirrors := ParseURLArg(arg)
			if url == "" {
				continue
			}
			err := sendToServer(url, mirrors, outputDir, filename, forceSingle, chunkCount, port)
			if err != nil {
				fmt.Printf("Error adding %s: %v\n", url, err)
			} else {
				successCount++
			}
		}
		return successCount
	}

	// Internal add (TUI or headless mode).
	if GlobalService == nil {
		fmt.Fprintln(os.Stderr, "Error: GlobalService not initialized")
		return 0
	}

	settings, err := config.LoadSettings()
	if err != nil {
		settings = config.DefaultSettings()
	}

	for _, arg := range urls {
		// Validation
		if arg == "" {
			continue
		}

		url, mirrors := ParseURLArg(arg)
		if url == "" {
			continue
		}

		// Prepare output path.
		outPath := outputDir
		if outPath == "" {
			if settings.General.DefaultDownloadDir != "" {
				outPath = settings.General.DefaultDownloadDir
				_ = os.MkdirAll(outPath, 0o755)
			} else {
				outPath = "."
			}
		}
		outPath = utils.EnsureAbsPath(outPath)

		// Check for duplicates/extensions if we are in TUI mode (serverProgram != nil)
		// For headless/root direct add, we might skip prompt or auto-approve?
		// For now, let's just add directly if headless, or prompt if TUI is up.

		// If TUI is up (serverProgram != nil), we might want to send a request msg?
		// But processDownloads is called from QUEUE init routine, primarily for CLI args.
		// If CLI args provided, user probably wants them added immediately.

		var opts *types.AddOptions
		if forceSingle || chunkCount > 0 {
			opts = &types.AddOptions{ForceSingle: forceSingle, ChunkCount: chunkCount}
		}
		_, err := GlobalService.Add(url, outPath, filename, mirrors, nil, opts)
		if err != nil {
			fmt.Printf("Error adding %s: %v\n", url, err)
			continue
		}
		atomic.AddInt32(&activeDownloads, 1)
		successCount++
	}
	return successCount
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.Flags().StringP("batch", "b", "", "File containing URLs to download (one per line)")
	rootCmd.Flags().IntP("port", "p", 0, "Port to listen on (default: 8080 or first available)")
	rootCmd.Flags().StringP("output", "o", "", "Default output directory")
	rootCmd.Flags().StringP("filename", "n", "", "Override output filename (single URL only)")
	rootCmd.Flags().Bool("clipboard", false, "Read URL from clipboard")
	rootCmd.Flags().Bool("force-single", false, "Force single-connection downloader")
	rootCmd.Flags().Int("chunks", 0, "Override number of chunks/connections for this download")
	rootCmd.Flags().Bool("no-resume", false, "Do not auto-resume paused downloads on startup")
	rootCmd.Flags().Bool("exit-when-done", false, "Exit when all downloads complete")
	rootCmd.SetVersionTemplate("GoFetch v{{.Version}}\n")
}

// initializeGlobalState prepares directories, DB, and logging for CLI usage.
func initializeGlobalState() {

	stateDir := config.GetStateDir()
	logsDir := config.GetLogsDir()

	// Ensure directories exist
	_ = os.MkdirAll(stateDir, 0o755)
	_ = os.MkdirAll(logsDir, 0o755)

	// Config engine state
	state.Configure(filepath.Join(stateDir, "GoFetch.db"))

	// Config logging
	utils.ConfigureDebug(logsDir)

	// Clean up old logs
	settings, err := config.LoadSettings()
	var retention int
	if err == nil {
		retention = settings.General.LogRetentionCount
	} else {
		retention = config.DefaultSettings().General.LogRetentionCount
	}
	utils.CleanupLogs(retention)
}

// resumePausedDownloads honors settings to auto-resume saved downloads.
func resumePausedDownloads() {
	settings, err := config.LoadSettings()
	if err != nil {
		return // Can't check preference
	}

	pausedEntries, err := state.LoadPausedDownloads()
	if err != nil {
		return
	}

	for _, entry := range pausedEntries {
		// If entry is explicitly queued, we should start it regardless of AutoResume setting
		// If entry is paused, we only start it if AutoResume is enabled
		if entry.Status == "paused" && !settings.General.AutoResume {
			continue
		}
		if GlobalService == nil || entry.ID == "" {
			continue
		}
		if err := GlobalService.Resume(entry.ID); err == nil {
			atomic.AddInt32(&activeDownloads, 1)
		}
	}
}
