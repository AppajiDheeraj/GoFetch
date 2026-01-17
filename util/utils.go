// Package util provides utility functions used across the downloader application.
// It contains helper functions for common operations like extracting filenames from URLs
// and defines shared constants. This package centralizes reusable functionality to avoid
// code duplication and maintain consistency.
package util

import (
	"fmt"
	"net/url"
	"path"
)

// ExtractFileName parses the URL and extracts the filename from the path component.
// It returns the filename or an error if the filename cannot be determined.
func ExtractFileName(urlStr string) (string, error) {
	parsedURL, err := url.Parse(urlStr)

	if err != nil {
		return "", err
	}

	fileName := path.Base(parsedURL.Path)
	if fileName == "/" || fileName == "." {
		return "", fmt.Errorf("Unable to extract file name from URL: %s", urlStr)
	}

	return fileName, nil
}
