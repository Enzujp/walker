package cmd

import (
	"fmt"
	"github.com/enzujp/walker/example"
	"github.com/enzujp/walker/internal/generator"
	"github.com/enzujp/walker/internal/registry"
	"github.com/spf13/cobra"
)

var extractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract routes from your app",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Extracting routes...")

		// setup router ( register routes )
		example.SetupRouter()

		// print out registered metadata
		collection, err := generator.GeneratePostmanCollection(registry.Routes)
		if err != nil {
			fmt.Println("Error generating collection: ", err)
			return
		}
		fmt.Println(string(collection))
	},
}

func init() {
	rootCmd.AddCommand(extractCmd)
}
