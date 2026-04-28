package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var rootCmd = &cobra.Command{
	Use:   "autodoc",
	Short: "Auto generate Open API specs for running your code",
	Long:  "A tool that extracts routes and generates Postman/OpenAPI specs automatically.",
	//Run: func(cmd *cobra.Command, args []string) {
	//	fmt.Println("Use a subcommand like 'extract'")
	//},
}

func Execute() {
	if execErr := rootCmd.Execute(); execErr != nil {
		fmt.Println(execErr)
		os.Exit(1)
	}
}
