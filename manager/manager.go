// Package manager orchestrates the concurrent file download process.
// It coordinates the workflow: fetching file metadata, splitting downloads into chunks,
// managing parallel downloads, merging chunks, and cleanup. This package serves as the
// main controller for the downloader application.
package manager

import (
	"concurrent_downloader/greenhttp"
	"concurrent_downloader/models"
	"concurrent_downloader/util"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"sync"
)

// Init initializes the downloader by printing the startup message of the day (MOTD).
func Init() {
	fmt.Println(util.InitMotd)
}

// End prints the end message of the day (MOTD) when the downloader finishes.
func End() {
	fmt.Println(util.EndMotd)
}

// Run orchestrates the concurrent download process for the given URL.
// It performs a HEAD request to get file metadata, splits the download into chunks,
// downloads each chunk concurrently, merges the chunks, and cleans up temporary files.
func Run(urlPtr *url.URL) {
	client := greenhttp.NewHTTPClient()

	url := urlPtr.String()
	method := "HEAD"
	headers := map[string]string{
		"User-Agent": "CFD-Downloader",
	}

	resp, err := client.Do(method, url, headers)

	if err != nil {
		log.Fatal(err)
	}

	contentLength := resp.Header.Get(util.CONTENT_LENGTH_HEADER)
	contentLengthInBytes, err := strconv.Atoi(contentLength)

	if err != nil {
		log.Fatal("Unsupported file download type.... Empty size sent by server ", err)
	}

	log.Println("Content-Length:", contentLengthInBytes)

	fname, err := util.ExtractFileName(url)

	if err != nil {
		log.Fatal("Error extracting filename...")
	}

	log.Printf("FileName extracted: %v", fname)

	chunks := util.WORKER_ROUTINES
	log.Printf("Set %v parrallel workers/connections", chunks)

	chunkSize := contentLengthInBytes / chunks
	log.Println("Each chunk size: ", chunkSize)

	downReq := &models.DownloadRequest{
		Url:        url,
		FileName:   fname,
		Chunks:     chunks,
		Chunksize:  chunkSize,
		TotalSize:  contentLengthInBytes,
		HttpClient: client,
	}

	byteRangeArray := make([][2]int, chunks)
	byteRangeArray = downReq.SplitIntoChunks()
	fmt.Println(byteRangeArray)

	var wg sync.WaitGroup
	for idx, byteChunk := range byteRangeArray {
		wg.Add(1)

		go func(idx int, byteChunk [2]int) {
			defer wg.Done()
			err := downReq.Download(idx, byteChunk)
			if err != nil {
				log.Printf("Chunk %d failed: %v", idx, err)
				return
			}
		}(idx, byteChunk)
	}

	wg.Wait()

	err = downReq.MergeDownloads()

	if err != nil {
		log.Fatal("Failed merging tmp download files...", err)
	}

	err = downReq.CleanUpTempFiles()

	if err != nil {
		log.Fatal("Failed cleaning tmp download files...", err)
	}

	log.Printf("File Generated: %v\n\n", downReq.FileName)

	wg.Wait()
}
