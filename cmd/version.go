package cmd

import (
	"fmt"

	"github.com/novr/bitrise-cli/internal/api"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the br version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("br %s\n", api.Version)
		},
	})
}
