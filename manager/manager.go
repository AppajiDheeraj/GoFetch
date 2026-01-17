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

func Init() {
	fmt.Println(util.InitMotd)
}

func End(){
	fmt.Println(util.EndMotd)
}

func Run(urlPtr *url.URL) {
	client := greenhttp.NewHTTPClient()

	url := urlPtr.String()
	method := "HEAD"
	headers := map[string]string{
		"User-Agent" : "CFD-Downloader",
	}

	resp, err := client.Do(method,url,headers)

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

	log.Printf("FileName extracted: %v",fname)

	chunks := util.WORKER_ROUTINES
	log.Printf("Set %v parrallel workers/connections", chunks)

	chunkSize := contentLengthInBytes / chunks
	log.Println("Each chunk size: ", chunkSize)

	downReq := &models.DownloadRequest{
		Url: url,
		FileName: fname,
		Chunks: chunks,
		Chunksize: chunkSize,
		TotalSize: contentLengthInBytes,
		HttpClient: client,
	}

	byteRangeArray := make([][2]int,chunks)
	byteRangeArray = downReq.SplitIntoChunks()
	fmt.Println(byteRangeArray)

	var wg sync.WaitGroup
	for idx, byteChunk := range byteRangeArray {
		wg.Add(1)
		
		go func(idx int, byteChunk [2]int){
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
		log.Fatal("Failed merging tmp download files...",err)
	}

	err = downReq.CleanUpTempFiles()

	if err != nil {
		log.Fatal("Failed cleaning tmp download files...",err)
	}

	log.Printf("File Generated: %v\n\n", downReq.FileName)

	wg.Wait()
}