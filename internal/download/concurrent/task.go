package concurrent

import (
	"concurrent_downloader/internal/download/types"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// ActiveTask tracks a task currently being processed by a worker and exposes
// enough state to support health checks and work stealing.
type ActiveTask struct {
	Task          types.Task
	CurrentOffset int64
	StopAt        int64

	// Health monitoring fields
	LastActivity int64
	Speed        float64
	StartTime    time.Time
	Cancel       context.CancelFunc
	SpeedMu      sync.Mutex

	// Sliding window for recent speed tracking
	WindowStart time.Time
	WindowBytes int64

	Hedged int32 // Atomic: 1 if an idle worker is already racing this task
}

func (at *ActiveTask) RemainingBytes() int64 {
	current := atomic.LoadInt64(&at.CurrentOffset)
	stopAt := atomic.LoadInt64(&at.StopAt)
	if current >= stopAt {
		return 0
	}
	return stopAt - current
}

func (at *ActiveTask) RemainingTask() *types.Task {
	current := atomic.LoadInt64(&at.CurrentOffset)
	stopAt := atomic.LoadInt64(&at.StopAt)
	if current >= stopAt {
		return nil
	}
	return &types.Task{Offset: current, Length: stopAt - current}
}

func (at *ActiveTask) GetSpeed() float64 {
	at.SpeedMu.Lock()
	speed := at.Speed
	at.SpeedMu.Unlock()

	// Check for stall and decay speed to reflect idle connections.
	lastActivity := atomic.LoadInt64(&at.LastActivity)
	if lastActivity == 0 {
		return speed
	}

	since := time.Since(time.Unix(0, lastActivity))
	const decayThreshold = 2 * time.Second

	// If we haven't heard from the worker in > 2s, decay the speed to avoid
	// over-estimating throughput.
	if since > decayThreshold {
		decayFactor := float64(decayThreshold) / float64(since)
		speed *= decayFactor
	}

	return speed
}

func alignedSplitSize(remaining int64) int64 {
	half := (remaining / 2 / types.AlignSize) * types.AlignSize
	if half < types.MinChunk {
		return 0
	}
	return half
}
