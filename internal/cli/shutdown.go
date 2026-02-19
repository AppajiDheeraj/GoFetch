package cli

import (
	"concurrent_downloader/internal/utils"
	"fmt"
	"sync"
)

var (
	globalShutdownOnce sync.Once
	globalShutdownErr  error
	globalShutdownFn   = defaultGlobalShutdown
)

func defaultGlobalShutdown() error {
	// Prefer service shutdown to flush state and stop workers gracefully.
	if GlobalService != nil {
		return GlobalService.Shutdown()
	}
	if GlobalPool != nil {
		GlobalPool.GracefulShutdown()
	}
	return nil
}

func executeGlobalShutdown(reason string) error {
	// Ensure shutdown only happens once even if multiple signals arrive.
	globalShutdownOnce.Do(func() {
		utils.Debug("Executing graceful shutdown (%s)", reason)
		globalShutdownErr = globalShutdownFn()
		if globalShutdownErr != nil {
			globalShutdownErr = fmt.Errorf("graceful shutdown failed: %w", globalShutdownErr)
		}
	})
	return globalShutdownErr
}

func resetGlobalShutdownCoordinatorForTest(fn func() error) {
	globalShutdownOnce = sync.Once{}
	globalShutdownErr = nil
	if fn != nil {
		globalShutdownFn = fn
		return
	}
	globalShutdownFn = defaultGlobalShutdown
}
