Architecture
============

Overview
--------
GoFetch is a concurrent downloader with a clear separation between the CLI surface and the download engine. The CLI is only a thin orchestration layer; the core download functionality lives in the internal engine packages.

Key Packages
------------
- internal/cli: Cobra commands and user-facing flags. No download logic here.
- internal/download: The engine that manages probing, chunking, concurrent downloads, merge, and cleanup.
- internal/download/concurrent: Worker pool, task queue, and concurrent chunk downloads.
- internal/download/single: Single-connection fallback downloader.
- internal/events: Event types and stream helpers for progress and state changes.
- internal/state: Persistent state storage, resume support, and history.
- internal/config: Config and paths for runtime settings.
- internal/utils: Shared helpers (paths, filenames, logging, conversions).
- internal/core: Service interface and local service wiring.
- internal/clipboard: Clipboard validation utilities used by CLI.
- internal/probe.go: Server probe logic to determine size, filename, and range support.

Download Flow
-------------
1. CLI collects input and constructs options.
2. Probe reads headers and validates range support.
3. Manager selects concurrent vs single path based on probe results and config.
4. Concurrent downloader splits work into tasks and schedules workers.
5. Workers download ranges, emit events, and update state.
6. Manager merges parts, finalizes output, and cleans temporary files.

Concurrency Model
-----------------
- TaskQueue provides synchronized access to tasks and worker coordination.
- Worker pool consumes tasks in parallel and emits progress.
- Backpressure is controlled by the queue and worker count.

State and Events
----------------
- internal/state persists download metadata and resume data.
- internal/events publishes progress and lifecycle changes for CLI and services.

CLI vs Engine Boundary
----------------------
- CLI is responsible for argument parsing, UX, and invoking the engine.
- Engine is responsible for all download behavior, retries, and state updates.
