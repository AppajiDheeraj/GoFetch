package utils

import (
	"concurrent_downloader/internal/config"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	debugFile *os.File
	debugOnce sync.Once
	logsDir   atomic.Value // string
	verbose   atomic.Bool
)

func ConfigureDebug(dir string) {
	logsDir.Store(dir)
}

// SetVerbose enables or disables verbose logging
func SetVerbose(enabled bool) {
	verbose.Store(enabled)
}

// IsVerbose returns true if verbose logging is enabled
func IsVerbose() bool {
	return verbose.Load()
}

func Debug(format string, args ...any) {
	if !IsVerbose() {
		return
	}
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	debugOnce.Do(func() {
		logsDir := config.GetLogsDir()
		os.MkdirAll(logsDir, 0755)
		debugFile, _ = os.Create(filepath.Join(logsDir, fmt.Sprintf("debug-%s.log", time.Now().Format("20060102-150405"))))
	})
	if debugFile != nil {
		fmt.Fprintf(debugFile, "[%s] %s\n", timestamp, fmt.Sprintf(format, args...))
	}
}

// CleanupLogs removes old log files, keeping only the most recent retentionCount files
func CleanupLogs(retentionCount int) {
	if retentionCount < 0 {
		return // Keep all logs
	}

	val := logsDir.Load()
	if val == nil {
		return
	}
	dir := val.(string)

	if dir == "" {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		// If directory doesn't exist, nothing to clean
		return
	}

	var logs []fs.DirEntry
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasPrefix(entry.Name(), "debug-") && strings.HasSuffix(entry.Name(), ".log") {
			logs = append(logs, entry)
		}
	}

	// Sort by modification time (newest first)
	// Filenames have timestamp: debug-YYYYMMDD-HHMMSS.log, so alphabetical sort is also chronological
	// But let's rely on ModTime to be safe if possible, or just name since it is consistent
	sort.Slice(logs, func(i, j int) bool {
		// Newest first
		// Since names are YYYYMMDD-HHMMSS, reverse alphabetical works
		return logs[i].Name() > logs[j].Name()
	})

	if len(logs) <= retentionCount {
		return
	}

	// Remove older logs
	for _, log := range logs[retentionCount:] {
		path := filepath.Join(dir, log.Name())
		_ = os.Remove(path)
	}
}
