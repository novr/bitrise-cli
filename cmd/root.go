package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
	// Cancel in-flight requests on Ctrl+C / SIGTERM so commands abort promptly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
