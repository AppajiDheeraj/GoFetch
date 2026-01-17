// Package main is the entry point of the concurrent downloader application.
// It initializes the downloader, prompts the user for a URL, and orchestrates
// the download process through the manager package.
package main

import (
	"concurrent_downloader/cli"
	"concurrent_downloader/manager"
	"log"
)

func main() {
	manager.Init()
	defer manager.End()

	url, err := cli.GetURLFromUser()

	if err != nil {
		log.Fatal("Invalid URL:", err)
	}

	manager.Run(url)
}
