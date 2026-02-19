# 🚀 GoFetch

<div align="center">
<img src="./Logo.svg" alt="GoFetch Logo" width="600" />

![GoFetch Banner](https://img.shields.io/badge/GoFetch-Concurrent%20File%20Downloader-blue?style=for-the-badge)

**A lightning-fast concurrent file downloader built with Go**

[![Go Version](https://img.shields.io/badge/Go-1.24.5-00ADD8?style=flat-square&logo=go)](https://golang.org/)
[![Release](https://img.shields.io/github/v/release/AppajiDheeraj/GoFetch?style=flat-square)](https://github.com/AppajiDheeraj/GoFetch/releases)
[![License](https://img.shields.io/badge/license-MIT-green?style=flat-square)](LICENSE)
[![Build Status](https://img.shields.io/github/actions/workflow/status/AppajiDheeraj/GoFetch/release.yml?style=flat-square)](https://github.com/AppajiDheeraj/GoFetch/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/AppajiDheeraj/GoFetch?style=flat-square)](https://goreportcard.com/report/github.com/AppajiDheeraj/GoFetch)

<h4>
  <a href="https://github.com/AppajiDheeraj/GoFetch/issues/new?template=bug_report.yml">Report Bug</a>
·    
  <a href="https://github.com/AppajiDheeraj/GoFetch/issues/new?template=feature_request.yml">Request Feature</a>
</h4>
</div>

---

## 📖 Overview

GoFetch is a high-performance concurrent file downloader that accelerates your downloads by splitting files into multiple chunks and downloading them in parallel. Built with Go's powerful concurrency primitives, it provides a simple command-line interface for downloading files faster than traditional single-threaded downloaders.

## ✨ Features

- **⚡ Concurrent Downloads**: Splits files into multiple chunks and downloads them simultaneously using 10 parallel workers
- **🔄 HTTP Range Support**: Utilizes HTTP Range requests for efficient partial content downloads
- **🧩 Automatic Merging**: Seamlessly merges downloaded chunks into the final file
- **🧹 Smart Cleanup**: Automatically removes temporary files after successful download
- **📊 Progress Tracking**: Real-time logging of download progress for each chunk
- **🛡️ Error Handling**: Robust error handling and recovery mechanisms
- **🌐 Universal Compatibility**: Works with any HTTP/HTTPS URL that supports range requests
- **🎯 Simple CLI**: Easy-to-use command-line interface

## 🎬 Demo

```bash
$ gofetch
Enter the file URL to download: https://example.com/largefile.zip

Content-Length: 104857600
FileName extracted: largefile.zip
Set 10 parallel workers/connections
Each chunk size: 10485760

Downloading chunk 0
Downloading chunk 1
Downloading chunk 2
...
[SUCCESS] Chunk Written to file : 9
[SUCCESS] File chunks merged Successfully ...
File Generated: largefile.zip

THANK YOU!
```

## 🔧 Installation

### Prerequisites

- Go 1.24.5 or higher
- Internet connection

### From Source

```bash
# Clone the repository
git clone https://github.com/AppajiDheeraj/GoFetch.git

# Navigate to the project directory
cd GoFetch

# Build the binary
go build -o gofetch

# Run the application
./gofetch
```

### Using Go Install

```bash
go install github.com/AppajiDheeraj/GoFetch@latest
```

### Pre-built Binaries

Download pre-built binaries for your platform from the [Releases](https://github.com/AppajiDheeraj/GoFetch/releases) page.

Available for:
- **Linux** (amd64, arm64)
- **macOS** (amd64, arm64)
- **Windows** (amd64, arm64)

## 🚀 Usage

### Basic Usage

Simply run the binary and enter the URL when prompted:

```bash
./gofetch
```

Then enter the file URL:

```
Enter the file URL to download: https://example.com/file.zip
```

## 🧩 Public API (pkg/gofetch)

GoFetch exposes a stable, embeddable API for other Go apps via the `pkg/gofetch` package.

Install:

```bash
go get github.com/AppajiDheeraj/GoFetch@latest
```

Example:

```go
package main

import (
  "context"
  "log"

  "github.com/AppajiDheeraj/GoFetch/pkg/gofetch"
)

func main() {
  client, err := gofetch.NewClient(&gofetch.ClientOptions{Verbose: true})
  if err != nil {
    log.Fatal(err)
  }
  defer func() {
    _ = client.Shutdown()
  }()

  _, err = client.Add("https://example.com/file.zip", "./downloads", "", nil)
  if err != nil {
    log.Fatal(err)
  }

  stream, cleanup, err := client.StreamEvents(context.Background())
  if err != nil {
    log.Fatal(err)
  }
  defer cleanup()

  for msg := range stream {
    _ = msg // handle events (DownloadStartedMsg, ProgressMsg, etc.)
  }
}
```

### How It Works

1. **URL Input**: Enter the URL of the file you want to download
2. **Metadata Retrieval**: GoFetch performs a HEAD request to get file size and metadata
3. **Chunk Calculation**: The file is divided into 10 equal chunks
4. **Concurrent Download**: Each chunk is downloaded simultaneously using goroutines
5. **Merging**: All chunks are merged into the final file
6. **Cleanup**: Temporary chunk files are automatically removed

## 🏗️ Architecture

```
GoFetch/
├── cli/              # Command-line interface handling
│   └── cli.go        # User input and URL parsing
├── greenhttp/        # HTTP client implementation
│   └── http_client.go # Custom HTTP client with range support
├── manager/          # Download orchestration
│   └── manager.go    # Main download logic and coordination
├── models/           # Data models
│   └── download.go   # Download request model and methods
├── util/             # Utility functions
│   ├── const.go      # Constants and configuration
│   └── utils.go      # Helper functions
└── main.go           # Application entry point
```

### Key Components

- **CLI**: Handles user interaction and URL validation
- **HTTP Client**: Custom HTTP client with support for range requests and headers
- **Manager**: Orchestrates the download process, chunk distribution, and merging
- **Models**: Defines the download request structure and methods for chunk handling
- **Utils**: Provides utility functions like filename extraction and constants

## ⚙️ Configuration

You can customize the number of parallel workers by modifying the `WORKER_ROUTINES` constant in `util/const.go`:

```go
const WORKER_ROUTINES = 10 // Adjust based on your needs
```

## 📋 Requirements

- The target server must support HTTP Range requests (most modern servers do)
- Sufficient disk space for temporary chunks and the final file
- Stable internet connection for optimal performance

## 🤝 Contributing

Contributions are welcome! Here's how you can help:

1. Fork the repository
2. Create a new branch (`git checkout -b feature/amazing-feature`)
3. Make your changes
4. Commit your changes (`git commit -m 'Add some amazing feature'`)
5. Push to the branch (`git push origin feature/amazing-feature`)
6. Open a Pull Request

Please ensure your code follows Go best practices and includes appropriate comments.

## 📝 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Built with Go's powerful concurrency features
- Inspired by the need for faster file downloads
- Thanks to all contributors and users of GoFetch

## 📞 Contact

**Appaji Dheeraj**

- GitHub: [@AppajiDheeraj](https://github.com/AppajiDheeraj)
- Project Link: [https://github.com/AppajiDheeraj/GoFetch](https://github.com/AppajiDheeraj/GoFetch)

---

<div align="center">

**If you find GoFetch useful, please consider giving it a ⭐!**

Made with ❤️ by Appaji Dheeraj

</div>

Here is a senior‑engineer plan for adding adaptive worker scaling with TCP‑style AIMD control, without changing any files yet.

Goal
Add a self‑learning control loop that continuously tunes worker count, chunk size, and retry strategy using signals: throughput, RTT, packet loss, and chunk completion time. The system should converge quickly on good networks and remain stable on poor ones.

Design Principles

Keep the control loop isolated behind a small interface.
Do not intertwine metrics collection with download logic.
Make changes incremental: metrics → controller → actuator.
Ensure safe defaults and rate‑limited adjustments.
Plan (High Level)

Metrics Instrumentation (non-invasive)

Define a central metrics struct with rolling windows:
throughput_bps
rtt_ms (if RTT is not explicit, approximate with time-to-first-byte or time-to-first-chunk-ack)
loss_rate (based on retries / failed chunk attempts)
chunk_completion_ms (duration of successful chunk)
Collect at: worker completion, retry, failure, and scheduler assignment.
Use exponential moving averages (EMA) + a short window for recent variance.
Control Loop (AIMD)

Run on a fixed interval (e.g., every 1–2 seconds).
AIMD rules for worker count:
Additive Increase: If loss low, RTT stable, throughput rising → workers += 1
Multiplicative Decrease: If loss high or RTT spikes → workers = max(minWorkers, floor(workers * beta)) where beta ≈ 0.5–0.7
AIMD rules for chunk size:
Increase slowly when RTT stable and loss low.
Decrease aggressively when loss or retry spikes.
Retry strategy:
If loss rises, increase backoff and reduce max concurrency.
If network stable, lower backoff and allow more aggressive retries.
Actuation Layer

Provide a controller interface such as:
RecommendWorkers()
RecommendChunkSize()
RecommendRetryPolicy()
Scheduler consumes these recommendations but retains final say to avoid rapid oscillation.
Rate-limit changes (e.g., max 1 worker change per control tick).
Safety & Stability

Use bounds: minWorkers, maxWorkers, minChunkSize, maxChunkSize.
Add hysteresis to avoid thrashing.
Store last 2–3 decisions; only apply if consistent.
Ensure backward compatibility with current behavior when metrics are absent.
Testing Strategy

Unit tests for AIMD decisions with synthetic metrics.
Simulation tests with predefined scenarios:
good Wi‑Fi (low loss, stable RTT)
hotspot (bursty loss, RTT spikes)
slow DSL (high RTT, low throughput)
Benchmarks for regressions.
Observability

Log control decisions with reason codes:
INC_WORKERS, DEC_WORKERS, INC_CHUNK, DEC_CHUNK, RETRY_BACKOFF_UP
Add optional debug mode to print the control loop summary.
Deliverables

A controller package that can be reused.
Metrics pipeline with small, explicit hooks.
Stability constraints and documented parameters.
A short doc section: “Adaptive Scaling (AIMD)”.
Proposed Sequence of Work (No Code Yet)

Identify points to instrument metrics.
Define metrics struct and EMA helpers.
Implement control loop + AIMD rules.
Wire in scheduler/worker pool hooks.
Add safety limits and logging.
Tests + docs.