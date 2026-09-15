package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/enzujp/walker/pkg/walker"
	"github.com/spf13/cobra"
)

// ErrRoutesRemoved indicates a completed diff that violated --fail-on-removed.
var ErrRoutesRemoved = errors.New("endpoint removal policy failed")

// ExitCode distinguishes operational errors (1) from removal policy failures (2).
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if errors.Is(err, ErrRoutesRemoved) {
		return 2
	}
	return 1
}

func newDiffCommand() *cobra.Command {
	var format string
	var failOnRemoved bool
	command := &cobra.Command{
		Use:   "diff PREVIOUS CURRENT",
		Short: "Compare endpoint methods and paths in two JSON route manifests",
		Long:  "Compare endpoint methods and paths in two JSON route manifests. Use - for one stdin input. Metadata changes are ignored; path or method changes appear as a removal and an addition.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if format != "text" && format != "json" {
				return fmt.Errorf("unsupported diff format %q: use text or json", format)
			}
			if args[0] == "-" && args[1] == "-" {
				return fmt.Errorf("only one manifest may use stdin")
			}
			previous, err := readManifest(args[0], cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("previous: %w", err)
			}
			current, err := readManifest(args[1], cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("current: %w", err)
			}
			changes, err := walker.Diff(previous, current)
			if err != nil {
				return err
			}
			var output string
			if format == "json" {
				data, err := json.MarshalIndent(changes, "", "  ")
				if err != nil {
					return err
				}
				output = string(data) + "\n"
			} else {
				var report strings.Builder
				for _, e := range changes.Added {
					fmt.Fprintf(&report, "ADDED    %s %s\n", e.Method, e.Path)
				}
				for _, e := range changes.Removed {
					fmt.Fprintf(&report, "REMOVED  %s %s\n", e.Method, e.Path)
				}
				if report.Len() == 0 {
					report.WriteString("No endpoint changes.\n")
				}
				output = report.String()
			}
			if _, err := fmt.Fprint(cmd.OutOrStdout(), output); err != nil {
				return err
			}
			if failOnRemoved && len(changes.Removed) > 0 {
				return ErrRoutesRemoved
			}
			return nil
		},
	}
	command.Flags().StringVarP(&format, "format", "f", "text", "Report format: text or json")
	command.Flags().BoolVar(&failOnRemoved, "fail-on-removed", false, "Exit with code 2 when endpoints have been removed")
	return command
}
