package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// GetGoFetchDir returns the per-user config root based on OS conventions.
func GetGoFetchDir() string {
	switch runtime.GOOS {
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = filepath.Join(os.Getenv("USERPROFILE"), "AppData", "Roaming")
		}
		return filepath.Join(appData, "GoFetch")
	case "darwin": //MacOS
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "GoFetch")
	default: //Linux
		configHome := os.Getenv("XDG_CONFIG_HOME")
		if configHome == "" {
			home, _ := os.UserHomeDir()
			configHome = filepath.Join(home, ".config")
		}
		return filepath.Join(configHome, "GoFetch")
	}
}

// GetRuntimeDir returns the directory for runtime files (pid, port, lock).
// Linux: $XDG_RUNTIME_DIR/GoFetch or fallback to GetStateDir() if unset
// macOS: $TMPDIR/GoFetch-runtime
// Windows: %TEMP%/GoFetch
func GetRuntimeDir() string {
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(os.TempDir(), "GoFetch")
	case "darwin":
		return filepath.Join(os.TempDir(), "GoFetch-runtime")
	default: // Linux
		runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
		if runtimeDir != "" {
			return filepath.Join(runtimeDir, "GoFetch")
		}
		// Fallback to state dir if XDG_RUNTIME_DIR is not set (e.g. docker, headless)
		return GetStateDir()
	}
}

// EnsureAbsPath normalizes a path for consistent state lookups.
func EnsureAbsPath(path string) string {
	if path == "" {
		path = "."
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

// GetStateDir returns the directory for persistent state (DB, tokens).
func GetStateDir() string {
	return filepath.Join(GetGoFetchDir(), "state")
}

// GetLogsDir returns the directory for logs.
func GetLogsDir() string {
	return filepath.Join(GetGoFetchDir(), "logs")
}

// EnsureDirs creates all required directories.
func EnsureDirs() error {
	dirs := []string{GetGoFetchDir(), GetStateDir(), GetLogsDir(), GetRuntimeDir()}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return nil
}
