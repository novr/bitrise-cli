package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "br",
	Short: "Bitrise CLI",
	Long:  "Access Bitrise build history and logs from your terminal.",
	// Execute() prints the error itself; silence cobra's own reporting so it
	// isn't printed twice.
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
