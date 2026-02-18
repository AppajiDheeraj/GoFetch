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
	if GlobalService != nil {
		return GlobalService.Shutdown()
	}
	if GlobalPool != nil {
		GlobalPool.GracefulShutdown()
	}
	return nil
}

func executeGlobalShutdown(reason string) error {
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
