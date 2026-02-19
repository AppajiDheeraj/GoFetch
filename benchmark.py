import argparse
import os
import shlex
import subprocess
import time


def run_cmd(cmd: list[str]) -> float:
	"""Run a command and return elapsed wall time in seconds."""
	start = time.monotonic()
	subprocess.run(cmd, check=True)
	return time.monotonic() - start


def main() -> int:
	"""Compare single-connection vs concurrent download performance."""
	parser = argparse.ArgumentParser(description="Benchmark GoFetch single vs concurrent")
	parser.add_argument(
		"--url",
		default="http://commondatastorage.googleapis.com/gtv-videos-bucket/sample/BigBuckBunny.mp4",
		help="File URL to download",
	)
	parser.add_argument(
		"--cmd",
		default="go run main.go",
		help="Base command to run GoFetch (e.g. 'go run main.go' or '.\\GoFetch.exe')",
	)
	parser.add_argument("--outdir", default=".", help="Output directory")
	args = parser.parse_args()

	os.makedirs(args.outdir, exist_ok=True)
	# Derive a stable filename so single/concurrent outputs are comparable.
	filename = os.path.basename(args.url.split("?")[0]) or "download.bin"

	base_cmd = shlex.split(args.cmd)

	# Force single-connection mode to isolate strategy differences.
	single_cmd = (
		base_cmd
		+ [
			"--force-single",
			"--exit-when-done",
			"--output",
			args.outdir,
			"--filename",
			f"single_{filename}",
			args.url,
		]
	)

	# Default mode uses the concurrent downloader.
	concurrent_cmd = (
		base_cmd
		+ [
			"--exit-when-done",
			"--output",
			args.outdir,
			"--filename",
			f"concurrent_{filename}",
			args.url,
		]
	)

	print(f"URL: {args.url}")
	print(f"Single command: {' '.join(single_cmd)}")
	print(f"Concurrent command: {' '.join(concurrent_cmd)}")

	print("Running single download (forced single)...")
	single_time = run_cmd(single_cmd)
	print(f"Single time: {single_time:.2f}s")

	print("Running concurrent download...")
	concurrent_time = run_cmd(concurrent_cmd)
	print(f"Concurrent time: {concurrent_time:.2f}s")

	if concurrent_time > 0:
		ratio = single_time / concurrent_time
		print(f"Speedup: {ratio:.2f}x")

	return 0


if __name__ == "__main__":
	raise SystemExit(main())
