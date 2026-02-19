package cli

import (
	"concurrent_downloader/internal/clipboard"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var addCmd = &cobra.Command{
	Use:     "add [url]...",
	Aliases: []string{"get"},
	Short:   "Add a new download to the running GoFetch instance",
	Long:    `Add one or more URLs to the download queue of a running GoFetch instance.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Initialize Global State (needed for config/paths)
		initializeGlobalState()

		batchFile, _ := cmd.Flags().GetString("batch")
		output, _ := cmd.Flags().GetString("output")
		clipboardFlag, _ := cmd.Flags().GetBool("clipboard")
		filename, _ := cmd.Flags().GetString("filename")
		forceSingle, _ := cmd.Flags().GetBool("force-single")

		// Collect URLs from multiple sources to keep CLI UX simple.
		var urls []string

		// 1. URLs from args.
		urls = append(urls, args...)

		// 2. URLs from clipboard.
		if clipboardFlag {
			url, err := clipboard.ReadURL()
			if err != nil {
				if err == clipboard.ErrInvalidURL {
					fmt.Fprintln(os.Stderr, "Error: Clipboard does not contain a valid URL")
				} else {
					fmt.Fprintf(os.Stderr, "Error reading from clipboard: %v\n", err)
				}
				os.Exit(1)
			}
			urls = append(urls, url)
			fmt.Printf("📋 URL from clipboard: %s\n", url)
		}

		// 3. URLs from batch file.
		if batchFile != "" {
			fileUrls, err := readURLsFromFile(batchFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reading batch file: %v\n", err)
				os.Exit(1)
			}
			urls = append(urls, fileUrls...)
		}

		if len(urls) == 0 {
			_ = cmd.Help()
			return
		}

		if filename != "" && len(urls) > 1 {
			fmt.Fprintln(os.Stderr, "Error: --filename can only be used with a single URL")
			os.Exit(1)
		}

		// Check if GoFetch is running to decide local vs remote add.
		port := readActivePort()
		if port == 0 {
			fmt.Println("Error: GoFetch is not running.")
			fmt.Println("Use 'GoFetch <url>' to start GoFetch with a download.")
			os.Exit(1)
		}

		// Send downloads to server
		count := processDownloads(urls, output, filename, forceSingle, port)

		if count > 0 {
			fmt.Printf("Successfully added %d downloads.\n", count)
		}
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
	addCmd.Flags().StringP("batch", "b", "", "File containing URLs to download (one per line)")
	addCmd.Flags().StringP("output", "o", "", "Output directory")
	addCmd.Flags().StringP("filename", "n", "", "Override output filename (single URL only)")
	addCmd.Flags().Bool("clipboard", false, "Read URL from clipboard")
	addCmd.Flags().Bool("force-single", false, "Force single-connection downloader")
}
