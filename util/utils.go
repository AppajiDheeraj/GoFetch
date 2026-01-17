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
