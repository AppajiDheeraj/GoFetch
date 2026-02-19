package main

import "concurrent_downloader/internal/cli"

func main() {
	// Delegate to cobra command tree.
	cli.Execute()
}
