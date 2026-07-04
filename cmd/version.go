package cmd

import (
	"fmt"

	"br/internal/api"

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
