package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/enzujp/walker/example"
	"github.com/enzujp/walker/pkg/walker"
	"github.com/spf13/cobra"
)

func newExtractCommand() *cobra.Command {
	var input, format, name, baseURL, config, bearerToken string
	var variables []string
	var demo, groupByPath bool
	command := &cobra.Command{
		Use:   "extract",
		Short: "Export a route manifest or the bundled Chi demo",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if demo == (input != "") {
				return fmt.Errorf("choose exactly one of --demo or --input (use - for stdin)")
			}
			if format != "postman" && format != "json" {
				return fmt.Errorf("unsupported format %q: use postman or json", format)
			}
			if format == "json" {
				for _, flag := range []string{"config", "name", "base-url", "bearer-token", "variable", "group-by-path"} {
					if cmd.Flags().Changed(flag) {
						return fmt.Errorf("--%s applies only to --format postman", flag)
					}
				}
			}
			var routes []walker.Route
			var err error
			if demo {
				routes, err = example.Routes()
			} else {
				routes, err = readManifest(input, cmd.InOrStdin())
			}
			if err != nil {
				return err
			}
			var output []byte
			if format == "json" {
				output, err = json.MarshalIndent(routes, "", "  ")
			} else {
				options := walker.Options{}
				if config != "" {
					options, err = readOptions(config)
					if err != nil {
						return err
					}
				}
				if cmd.Flags().Changed("name") {
					options.Name = name
				}
				if cmd.Flags().Changed("base-url") {
					options.BaseURL = baseURL
				}
				if cmd.Flags().Changed("group-by-path") {
					options.GroupByPath = groupByPath
				}
				if cmd.Flags().Changed("bearer-token") {
					options.Auth = &walker.Auth{Type: "bearer", Token: bearerToken}
				}
				if options.Variables == nil {
					options.Variables = map[string]string{}
				}
				seen := map[string]bool{}
				for _, entry := range variables {
					key, value, ok := strings.Cut(entry, "=")
					if !ok || key == "" {
						return fmt.Errorf("--variable must be KEY=VALUE")
					}
					if seen[key] {
						return fmt.Errorf("duplicate --variable %q", key)
					}
					seen[key] = true
					options.Variables[key] = value
				}
				output, err = walker.Postman(routes, options)
			}
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), string(output))
			return err
		},
	}
	command.Flags().BoolVar(&demo, "demo", false, "Export the bundled example router")
	command.Flags().StringVarP(&input, "input", "i", "", "Route manifest path, or - for stdin")
	command.Flags().StringVarP(&format, "format", "f", "postman", "Output format: postman or json")
	command.Flags().StringVar(&config, "config", "", "Postman options JSON file; explicit flags override its values")
	command.Flags().StringVar(&name, "name", "Walker API", "Postman collection name")
	command.Flags().StringVar(&baseURL, "base-url", "http://localhost:8080", "Postman base_url variable")
	command.Flags().StringVar(&bearerToken, "bearer-token", "", "Collection bearer authentication, e.g. '{{token}}'")
	command.Flags().StringArrayVar(&variables, "variable", nil, "Collection variable KEY=VALUE (repeatable)")
	command.Flags().BoolVar(&groupByPath, "group-by-path", false, "Group routes without explicit groups by their first static path segment")
	return command
}
