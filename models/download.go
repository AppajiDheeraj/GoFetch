package models

import (
	"concurrent_downloader/greenhttp"
	"concurrent_downloader/util"
	"fmt"
	"io"
	"log"
	"os"
)

type DownloadRequest struct {
	Url        string
	FileName   string
	Chunks     int
	Chunksize  int
	TotalSize  int
	HttpClient *greenhttp.HTTPClient
}

func (d *DownloadRequest) SplitIntoChunks() [][2]int {
	arr := make([][2]int, d.Chunks)

	for i := 0; i < d.Chunks; i++ {
		if i == 0 {
			arr[i][0] = 0
			arr[i][1] = d.Chunksize
		} else if i == d.Chunks-1 {
			arr[i][0] = arr[i-1][1] + 1
			arr[i][1] = d.TotalSize - 1
		} else {
			arr[i][0] = arr[i-1][1] + 1
			arr[i][1] = arr[i][0] + d.Chunksize	
		}
	}
	return arr
}

func (d *DownloadRequest) Download(idx int, byteChunk [2]int) error {
	log.Printf("Downloading chunk %v", idx)

	method := "GET"
	headers := map[string]string{
		"User-Agent": "CFD Downloader",
		"Range":      fmt.Sprintf("bytes=%v-%v", byteChunk[0], byteChunk[1]),
	}
	resp, err := d.HttpClient.Do(method, d.Url, headers)

	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()	

	if resp.StatusCode > 299 {
		return fmt.Errorf("[INVALID] Cant Process, response = %v", resp.StatusCode)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 206 {
		return fmt.Errorf("invalid response: %d", resp.StatusCode)
	}

	fname := (fmt.Sprintf("%v-%v.tmp", util.TMP_FILE_PREFIX, idx))
	file, err := os.Create(fname)
	if err != nil {
		return fmt.Errorf("[Creation FALED] file = %v din't get created", fname)
	}

	defer file.Close()

	_, err = io.Copy(file, resp.Body)

	if err != nil {
		return fmt.Errorf("[FAILED] Write failed to file : %v", fname)
	}

	log.Printf("[SUCCESS] Chunk Written to file : %v", idx)

	return nil
}

func (d *DownloadRequest) MergeDownloads() error {
	out, err := os.Create(d.FileName)
	if err != nil {
		return fmt.Errorf("[FAILED]: Creation of Output file: %v", err)
	}

	defer out.Close()

	for idx := 0; idx < d.Chunks; idx++ {
		fname := fmt.Sprintf("%v-%v.tmp", util.TMP_FILE_PREFIX, idx)
		in, err := os.Open(fname)

		if err != nil {
			return fmt.Errorf("[FAILED] Opening Output File : %v", fname)
		}

		_, err = io.Copy(out, in)

		in.Close()



		if err != nil {
			return fmt.Errorf("Failed Merging Chunk File %s to %v", fname, err)
		}
	}

	fmt.Println("[SUCCESS] File chunks merged Successfully ...")
	return nil
}

func (d *DownloadRequest) CleanUpTempFiles() error {
	log.Println("Starting to clean up the temp files")

	for idx := 0; idx < d.Chunks; idx++ {
		fname := fmt.Sprintf("%v-%v.tmp", util.TMP_FILE_PREFIX, idx)
		err := os.Remove(fname)

		if err != nil {
			return fmt.Errorf("Failed to remove chunk file %s: %v: ", fname, err)
		}
	}
	return nil
}
