package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var tokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Print the auth token used by the GoFetch daemon",
	Run: func(cmd *cobra.Command, args []string) {
		token := ensureAuthToken()
		fmt.Println(token)
	},
}

func init() {
	rootCmd.AddCommand(tokenCmd)
}
