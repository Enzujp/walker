// Package cmd implements Walker's command-line interface.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// NewCommand creates an independent command tree, safe to use in tests.
func NewCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "walker",
		Short:         "Export HTTP routes as JSON or a Postman collection",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newExtractCommand(), newDiffCommand())
	return root
}

func Execute() {
	if err := NewCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "walker:", err)
		os.Exit(ExitCode(err))
	}
}
