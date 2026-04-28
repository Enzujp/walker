package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
)

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract routes from your app",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Extracting routes...")
	},
}

func init() {
	rootCmd.AddCommand(extractCmd)
}
