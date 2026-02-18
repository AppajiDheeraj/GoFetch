package cli

import (
	"concurrent_downloader/internal/config"
	"concurrent_downloader/internal/core"
	"concurrent_downloader/internal/utils"
	"fmt"

	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Manage the GoFetch background server (daemon)",
	Long:  `Start, stop, or check the status of the GoFetch background server.`,
}

var serverStartCmd = &cobra.Command{
	Use:   "start [url]...",
	Short: "Start the GoFetch server in headless mode",
	Run: func(cmd *cobra.Command, args []string) {
		initializeGlobalState()

		// Attempt to acquire lock
		isMaster, err := AcquireLock()
		if err != nil {
			fmt.Printf("Error acquiring lock: %v\n", err)
			os.Exit(1)
		}

		if !isMaster {
			fmt.Fprintln(os.Stderr, "Error: GoFetch server is already running.")
			os.Exit(1)
		}
		defer func() {
			if err := ReleaseLock(); err != nil {
				utils.Debug("Error releasing lock: %v", err)
			}
		}()

		portFlag, _ := cmd.Flags().GetInt("port")
		batchFile, _ := cmd.Flags().GetString("batch")
		outputDir, _ := cmd.Flags().GetString("output")
		filename, _ := cmd.Flags().GetString("filename")
		forceSingle, _ := cmd.Flags().GetBool("force-single")
		exitWhenDone, _ := cmd.Flags().GetBool("exit-when-done")
		noResume, _ := cmd.Flags().GetBool("no-resume")

		// Save current PID to file
		savePID()
		defer removePID()

		// Determine Port
		// Determine Port
		// Logic moved to startServerLogic, or we need to pass flags.
		// Use startServerLogic
		startServerLogic(cmd, args, portFlag, batchFile, outputDir, filename, forceSingle, exitWhenDone, noResume)
	},
}

var serverStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the running GoFetch server",
	Run: func(cmd *cobra.Command, args []string) {
		pid := readPID()
		if pid == 0 {
			fmt.Println("No running GoFetch server found (PID file missing).")
			return
		}

		process, err := os.FindProcess(pid)
		if err != nil {
			fmt.Printf("Error finding process: %v\n", err)
			return
		}

		if runtime.GOOS == "windows" {
			if err := process.Kill(); err != nil {
				// Fallback to taskkill for Windows permissions/process tree cleanup
				cmd := exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/T", "/F")
				if runErr := cmd.Run(); runErr != nil {
					fmt.Printf("Error stopping server: %v\n", err)
					fmt.Println("If this persists, run the terminal as Administrator and retry 'GoFetch server stop'.")
					return
				}
				fmt.Println("Stopped server using taskkill.")
			}
			cleanupRuntimeFiles()
			fmt.Printf("Stopped server process %d\n", pid)
			return
		}

		// Try to send SIGTERM on Unix-like systems
		err = process.Signal(syscall.SIGTERM)
		if err != nil {
			fmt.Printf("Error stopping server: %v\n", err)
			return
		}

		fmt.Printf("Sent stop signal to process %d\n", pid)
	},
}

var serverStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check the status of the GoFetch server",
	Run: func(cmd *cobra.Command, args []string) {
		pid := readPID()
		if pid == 0 {
			fmt.Println("GoFetch server is NOT running.")
			return
		}

		// Check if process exists
		process, err := os.FindProcess(pid)
		if err != nil {
			fmt.Printf("GoFetch server is NOT running (Process %d not found).\n", pid)
			// Cleanup stale pid file?
			return
		}

		// Sending signal 0 to check existence
		err = process.Signal(syscall.Signal(0))
		if err != nil {
			fmt.Printf("GoFetch server is NOT running (Process %d dead).\n", pid)
			return
		}

		port := readActivePort()
		fmt.Printf("GoFetch server is running (PID: %d, Port: %d).\n", pid, port)
	},
}

func init() {
	rootCmd.AddCommand(serverCmd)
	serverCmd.AddCommand(serverStartCmd)
	serverCmd.AddCommand(serverStopCmd)
	serverCmd.AddCommand(serverStatusCmd)

	serverStartCmd.Flags().StringP("batch", "b", "", "File containing URLs to download")
	serverStartCmd.Flags().IntP("port", "p", 0, "Port to listen on")
	serverStartCmd.Flags().StringP("output", "o", "", "Default output directory")
	serverStartCmd.Flags().StringP("filename", "n", "", "Override output filename (single URL only)")
	serverStartCmd.Flags().Bool("force-single", false, "Force single-connection downloader")
	serverStartCmd.Flags().Bool("exit-when-done", false, "Exit when all downloads complete")
	serverStartCmd.Flags().Bool("no-resume", false, "Do not auto-resume paused downloads on startup")
}

func savePID() {
	pid := os.Getpid()
	pidFile := filepath.Join(config.GetRuntimeDir(), "pid")
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0o644); err != nil {
		utils.Debug("Error writing PID file: %v", err)
	}
}

func removePID() {
	pidFile := filepath.Join(config.GetRuntimeDir(), "pid")
	if err := os.Remove(pidFile); err != nil && !os.IsNotExist(err) {
		utils.Debug("Error removing PID file: %v", err)
	}
}

func readPID() int {
	pidFile := filepath.Join(config.GetRuntimeDir(), "pid")
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(string(data))
	return pid
}

func cleanupRuntimeFiles() {
	removePID()
	removeActivePort()
	lockFile := filepath.Join(config.GetRuntimeDir(), "GoFetch.lock")
	if err := os.Remove(lockFile); err != nil && !os.IsNotExist(err) {
		utils.Debug("Error removing lock file: %v", err)
	}
}

func startServerLogic(cmd *cobra.Command, args []string, portFlag int, batchFile string, outputDir string, filename string, forceSingle bool, exitWhenDone bool, noResume bool) {
	port, listener, err := bindServerListener(portFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	// Initialize Service
	GlobalService = core.NewLocalDownloadServiceWithInput(GlobalPool, GlobalProgressCh)

	saveActivePort(port)
	defer removeActivePort()

	go startHTTPServer(listener, port, outputDir, GlobalService)

	// Queue initial downloads
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
			processDownloads(urls, outputDir, filename, forceSingle, 0)
		}
	}()

	fmt.Printf("GoFetch %s running in server mode.\n", Version)
	host := getServerBindHost()
	fmt.Printf("Serving on %s:%d\n", host, port)
	fmt.Println("Press Ctrl+C to exit.")

	StartHeadlessConsumer()

	// Auto-resume paused downloads (unless --no-resume)
	if !noResume {
		resumePausedDownloads()
	}

	if exitWhenDone {
		exitWhenDoneCh := make(chan struct{}, 1)
		go func() {
			time.Sleep(2 * time.Second)
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				if atomic.LoadInt32(&activeDownloads) == 0 {
					if GlobalPool != nil && GlobalPool.ActiveCount() == 0 {
						select {
						case exitWhenDoneCh <- struct{}{}:
						default:
						}
						return
					}
				}
			}
		}()

		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
		defer signal.Stop(sigChan)

		select {
		case sig := <-sigChan:
			fmt.Printf("\nReceived %s. Shutting down...\n", sig)
			_ = executeGlobalShutdown(fmt.Sprintf("server signal: %s", sig))
		case <-exitWhenDoneCh:
			fmt.Println("All downloads finished. Exiting...")
			_ = executeGlobalShutdown("server: exit when done")
		}
		return
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigChan)
	sig := <-sigChan

	fmt.Printf("\nReceived %s. Shutting down...\n", sig)
	_ = executeGlobalShutdown(fmt.Sprintf("server signal: %s", sig))
}
