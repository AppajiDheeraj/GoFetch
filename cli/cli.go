// Package cli handles command-line interface interactions with the user.
// It provides functions for reading user input, parsing URLs, and other CLI-related operations.
// This package separates user interaction logic from the core download functionality.
package cli

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
)

func GetURLFromUser() (*url.URL, error) {
	// Create a new scanner that reads from standard input (keyboard)
	scanner := bufio.NewScanner(os.Stdin)

	// Prompt the user to enter a URL
	fmt.Printf("Enter the file URL to download: ")

	// Read one full line from stdin (waits until user presses Enter)
	scanner.Scan()

	// Retrieve the text that was read by scanner.Scan()
	userInput := scanner.Text()

	// Parse the user input string into a URL structure
	parsedURL, err := url.Parse(userInput)

	// If the URL parsing failed, return no URL and the error
	if err != nil {
		return nil, err
	}

	// If everything succeeded, return the parsed URL and no error
	return parsedURL, nil
}
