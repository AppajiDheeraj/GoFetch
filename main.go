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