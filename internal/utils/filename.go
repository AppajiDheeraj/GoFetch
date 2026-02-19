package utils

// Package utils provides utility functions used across the downloader application.
// It contains helper functions for common operations like extracting filenames from URLs
// and defines shared constants. This package centralizes reusable functionality to avoid
// code duplication and maintain consistency.

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/h2non/filetype"
	"github.com/vfaronov/httpheader"
)

// DetermineFilename extracts the filename from a URL and HTTP response,
// applying various heuristics. It returns the determined filename,
// a new io.Reader that includes any sniffed header bytes, and an error.
func DetermineFilename(rawurl string, resp *http.Response, verbose bool) (string, io.Reader, error) {
	parsed, err := url.Parse(rawurl)
	if err != nil {
		return "", nil, err
	}

	var candidate string

	// Strategy 1: Check Content-Disposition header (most reliable)
	if _, name, err := httpheader.ContentDisposition(resp.Header); err == nil && name != "" {
		candidate = name
		if verbose {
			fmt.Fprintf(os.Stderr, "Filename from Content-Disposition: %s\n", candidate)
		}
	}

	// Strategy 2: Check URL query parameters for filename hints
	if candidate == "" {
		q := parsed.Query()
		if name := q.Get("filename"); name != "" {
			candidate = name
			if verbose {
				fmt.Fprintf(os.Stderr, "Filename from query param 'filename': %s\n", candidate)
			}
		} else if name := q.Get("file"); name != "" {
			candidate = name
			if verbose {
				fmt.Fprintf(os.Stderr, "Filename from query param 'file': %s\n", candidate)
			}
		}
	}

	// Strategy 3: Extract from URL path as fallback
	if candidate == "" {
		candidate = filepath.Base(parsed.Path)
	}

	filename := sanitizeFilename(candidate)

	// Read first 512 bytes for MIME type detection and magic number analysis
	header := make([]byte, 512)

	n, rerr := io.ReadFull(resp.Body, header)
	if rerr != nil {
		if rerr == io.ErrUnexpectedEOF || rerr == io.EOF {
			header = header[:n]
		} else {
			return "", nil, fmt.Errorf("reading header: %w", rerr)
		}
	} else {
		header = header[:n]
	}

	// Reconstruct body by prepending the header bytes back to the response stream
	body := io.MultiReader(
		bytes.NewReader(header), // first part
		resp.Body,               // rest of file
	)

	if verbose {
		mimeType := http.DetectContentType(header)
		fmt.Fprintln(os.Stderr, "Detected MIME:", mimeType)

		if kind, _ := filetype.Match(header); kind != filetype.Unknown {
			fmt.Fprintln(os.Stderr, "Magic Type:", kind.Extension, kind.MIME)
		}
	}

	if candidate == "." && len(header) >= 30 && bytes.HasPrefix(header, []byte{0x50, 0x4B, 0x03, 0x04}) {
		nameLen := int(binary.LittleEndian.Uint16(header[26:28]))
		start := 30
		end := start + nameLen
		if end <= len(header) {
			zipName := string(header[start:end])
			if zipName != "" {
				filename = filepath.Base(zipName)
				if verbose {
					fmt.Fprintln(os.Stderr, "ZIP internal filename:", zipName)
				}
			}
		}
	}

	// Add file extension based on magic bytes if missing
	if filepath.Ext(filename) == "" {
		if kind, _ := filetype.Match(header); kind != filetype.Unknown {
			if kind.Extension != "" {
				filename = filename + "." + kind.Extension
				if verbose {
					fmt.Fprintf(os.Stderr, "Added extension from magic type: %s\n", kind.Extension)
				}
			}
		}
	}

	if filename == "" || filename == "." || filename == "/" {
		filename = "download.bin"
		if verbose {
			fmt.Fprintln(os.Stderr, "Falling back to default filename: download.bin")
		}
	}

	return filename, body, nil
}

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

// sanitizeFilename removes characters that are unsafe or invalid across platforms.
func sanitizeFilename(name string) string {
	// Replace backslashes with forward slashes first so filepath.Base treats them as separators
	name = strings.ReplaceAll(name, "\\", "/")
	name = filepath.Base(name)
	if name == "." {
		return name
	}
	if name == "/" || name == "\\" {
		return "_"
	}
	name = strings.TrimSpace(name)

	// Remove ANSI escape codes
	name = ansiRegex.ReplaceAllString(name, "")

	// Remove control characters
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)

	name = strings.ReplaceAll(name, "/", "_")
	// Additional standard replacements for windows/linux safety
	name = strings.ReplaceAll(name, ":", "_")
	name = strings.ReplaceAll(name, "*", "_")
	name = strings.ReplaceAll(name, "?", "_")
	name = strings.ReplaceAll(name, "\"", "_")
	name = strings.ReplaceAll(name, "<", "_")
	name = strings.ReplaceAll(name, ">", "_")
	name = strings.ReplaceAll(name, "|", "_")
	return name
}
