package utils

import "path/filepath"

// EnsureAbsPath normalizes a path for consistent persistence and resume logic.
func EnsureAbsPath(path string) string {
	if path == "" {
		path = "."
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}
