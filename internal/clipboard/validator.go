package clipboard

import (
	"errors"
	"net/url"
	"strings"

	"github.com/atotto/clipboard"
)

const (
	// MaxURLLength is the maximum allowed URL length to prevent processing extremely long strings
	maxURLLength = 2048
)

var (
	// ErrClipboardRead indicates an error reading from the clipboard
	ErrClipboardRead = errors.New("failed to read from clipboard")
	// ErrInvalidURL indicates the clipboard content is not a valid URL
	ErrInvalidURL = errors.New("clipboard does not contain a valid URL")
)

type Validator struct {
	allowedSchemes map[string]bool
}

func NewValidator() *Validator {
	// Restrict to HTTP/S to avoid unsafe schemes from clipboard.
	return &Validator{
		allowedSchemes: map[string]bool{"http": true, "https": true},
	}
}

// ExtractURL validates and extracts a URL from the given text
// Returns empty string if the text is not a valid HTTP/HTTPS URL
func (v *Validator) ExtractURL(text string) string {
	text = strings.TrimSpace(text)

	// Quick reject: empty, too long, or contains newlines
	if text == "" || len(text) > maxURLLength || strings.ContainsAny(text, "\n\r") {
		return ""
	}

	parsed, err := url.Parse(text)
	if err != nil {
		return ""
	}

	// Validate scheme, host presence, and host is not empty/whitespace
	if !v.allowedSchemes[parsed.Scheme] || parsed.Host == "" || strings.TrimSpace(parsed.Host) == "" {
		return ""
	}

	return parsed.String()
}

// ReadURL reads the clipboard and returns a valid URL if found
// Returns an error if clipboard reading fails or no valid URL is found
func ReadURL() (string, error) {
	text, err := clipboard.ReadAll()
	if err != nil {
		return "", ErrClipboardRead
	}

	validator := NewValidator()
	url := validator.ExtractURL(text)
	if url == "" {
		return "", ErrInvalidURL
	}

	return url, nil
}
