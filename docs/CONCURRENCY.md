Concurrency Notes
=================

Overview
--------
This document explains how the concurrent downloader balances work when some workers are slow or idle.

Protocol Selection
------------------
- In auto mode, the downloader prefers HTTP/3 first, then HTTP/1.1, and uses HTTP/2 as a fallback.
- This favors independent TCP connections for chunked downloads when HTTP/3 is unavailable or fails.
- You can override this order with the `protocol_preference` setting.

Work Stealing
-------------
- When a worker is idle, it can steal part of the remaining byte range from the busiest active worker.
- The victim task is split at an aligned boundary so both workers can continue without overlap.
- The original worker stops early at a new stop offset and the stolen range is pushed back to the queue.
- This improves throughput when chunk sizes are large or when worker speeds diverge.

Hedged Work (Duplicate Racing)
------------------------------
- If work stealing is not possible (chunks too small to split), the scheduler can hedge the most expensive active task.
- A duplicate task is queued to an idle worker, racing the original on a fresh HTTP connection.
- Writes use WriteAt and are idempotent, so duplicate writes do not corrupt the file.
- The winner finishes first; the other stops naturally when the queue drains or when its range is already complete.

Safety and Correctness
----------------------
- Stealing only reduces the original task range; it never extends beyond the original boundaries.
- Hedging never changes offsets; it only duplicates remaining work for a bounded range.
- Both strategies preserve correctness even under retries and mirror failover.
