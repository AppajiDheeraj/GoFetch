package models

import (
	"fmt"
	"log"
	"math"
	"time"
)

/*
Adaptive Chunking Engine
------------------------
Based on:
1. Li, K. (2017) - Parallel File Downloading (PD2 model)
2. Torroid Download Manager (2024)
3. TCP throughput optimization research

Key ideas:
- Chunk size adapts to observed throughput
- Worker count adapts to file size
- No fake math or guessed performance
- Designed for HTTP-based downloaders
*/

// ======================= CONSTANTS =======================

const (
	// TCP-optimal chunk size range
	MinChunkSize = 256 * 1024       // 256 KB
	MaxChunkSize = 2 * 1024 * 1024  // 2 MB

	// Worker limits
	MinWorkers = 2
	MaxWorkers = 128

	// File size thresholds
	SmallFileThreshold  = 10 * 1024 * 1024       // 10 MB
	MediumFileThreshold = 100 * 1024 * 1024      // 100 MB
	LargeFileThreshold  = 1024 * 1024 * 1024     // 1 GB
)

// ======================= DATA STRUCTS =======================

// Strategy returned by adaptive engine
type ChunkStrategy struct {
	FileSize     int
	ChunkSize    int
	ChunkCount   int
	WorkerCount  int
	StrategyName string
}

// AdaptiveChunker controls dynamic chunking
type AdaptiveChunker struct {
	FileSize int

	// Runtime adaptive state
	currentChunkSize int
	lastSpeed        float64
}

// ======================= CONSTRUCTOR =======================

func NewAdaptiveChunker(fileSize int) *AdaptiveChunker {
	return &AdaptiveChunker{
		FileSize:         fileSize,
		currentChunkSize: 512 * 1024, // Start at 512KB
	}
}

// ======================= STRATEGY LOGIC =======================

// Calculate initial strategy (before download starts)
func (a *AdaptiveChunker) CalculateInitialStrategy() *ChunkStrategy {
	var workers int
	var strategy string

	switch {
	case a.FileSize < SmallFileThreshold:
		workers = clamp(a.FileSize/(512*1024), 2, 4)
		strategy = "SMALL_FILE_MINIMAL"

	case a.FileSize < MediumFileThreshold:
		workers = clamp(a.FileSize/(1024*1024), 8, 16)
		strategy = "MEDIUM_FILE_BALANCED"

	case a.FileSize < LargeFileThreshold:
		workers = clamp(a.FileSize/(1024*1024), 32, 64)
		strategy = "LARGE_FILE_AGGRESSIVE"

	default:
		workers = clamp(a.FileSize/(1024*1024), 64, MaxWorkers)
		strategy = "HUGE_FILE_MAXIMUM"
	}

	chunkSize := a.FileSize / workers
	chunkSize = clamp(chunkSize, MinChunkSize, MaxChunkSize)

	return &ChunkStrategy{
		FileSize:     a.FileSize,
		ChunkSize:    chunkSize,
		ChunkCount:   int(math.Ceil(float64(a.FileSize) / float64(chunkSize))),
		WorkerCount:  workers,
		StrategyName: strategy,
	}
}

// ======================= ADAPTIVE LOGIC =======================

// Called after every chunk download
func (a *AdaptiveChunker) AdjustAfterChunk(bytesDownloaded int64, duration time.Duration) {
	if duration <= 0 {
		return
	}

	speed := float64(bytesDownloaded) / duration.Seconds()
	a.lastSpeed = speed

	// Adaptive chunk logic (Torroid + TCP behavior)
	switch {
	case speed > 6*1024*1024: // >6 MB/s
		a.currentChunkSize *= 2

	case speed < 1*1024*1024: // <1 MB/s
		a.currentChunkSize /= 2
	}

	a.currentChunkSize = clamp(a.currentChunkSize, MinChunkSize, MaxChunkSize)

	log.Printf(
		"[ADAPTIVE] speed=%.2f MB/s | newChunk=%d KB",
		speed/(1024*1024),
		a.currentChunkSize/1024,
	)
}

// Returns current chunk size
func (a *AdaptiveChunker) GetChunkSize() int {
	return a.currentChunkSize
}

// ======================= RANGE GENERATION =======================

func (a *AdaptiveChunker) GenerateRanges() [][2]int {
	chunkSize := a.currentChunkSize
	count := int(math.Ceil(float64(a.FileSize) / float64(chunkSize)))

	ranges := make([][2]int, count)

	for i := 0; i < count; i++ {
		start := i * chunkSize
		end := start + chunkSize - 1

		if end >= a.FileSize {
			end = a.FileSize - 1
		}

		ranges[i] = [2]int{start, end}
	}

	return ranges
}

// ======================= HELPERS =======================

func clamp(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// ======================= DEBUG =======================

func (s *ChunkStrategy) String() string {
	return fmt.Sprintf(
		"Strategy=%s | File=%dMB | Workers=%d | Chunk=%dKB",
		s.StrategyName,
		s.FileSize/(1024*1024),
		s.WorkerCount,
		s.ChunkSize/1024,
	)
}
