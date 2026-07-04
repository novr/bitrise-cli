package cmd

import (
	"fmt"
	"strconv"

	"br/internal/api"

	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "logs <build-number>",
		Short: "Show build logs",
		Args:  cobra.ExactArgs(1),
		Example: `  br build logs 123
  br build logs 123 --failed-only`,
		RunE: runBuildLogs,
	}
	cmd.Flags().Bool("failed-only", false, "Show only output from failed steps")
	buildCmd.AddCommand(cmd)
}

func runBuildLogs(cmd *cobra.Command, args []string) error {
	buildNumber, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid build number: %s", args[0])
	}
	failedOnly, _ := cmd.Flags().GetBool("failed-only")

	client, err := newAPIClient()
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	appSlug, err := resolveAppSlug(ctx, cmd.Parent(), client)
	if err != nil {
		return err
	}

	build, err := client.GetBuildByNumber(ctx, appSlug, buildNumber)
	if err != nil {
		return err
	}

	logText, archived, err := client.FetchLog(ctx, build.Slug)
	if err != nil {
		return fmt.Errorf("failed to fetch log: %w", err)
	}
	if logText == "" {
		if build.Status == api.StatusRunning {
			fmt.Println("Build is still starting, no logs available yet.")
		} else {
			fmt.Println("No logs available.")
		}
		return nil
	}
	if !archived && build.Status == api.StatusRunning {
		fmt.Printf("# Build #%d is still running — showing partial log\n\n", build.BuildNumber)
	}

	if failedOnly {
		filtered := failedStepLog(logText)
		if filtered == "" {
			fmt.Println("No failed steps detected in the log.")
		} else {
			fmt.Print(filtered)
		}
		return nil
	}

	fmt.Print(logText)
	return nil
}
