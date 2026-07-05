package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/novr/bitrise-cli/internal/api"

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

	logText, archived, err := client.FetchLog(ctx, appSlug, build.Slug)
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
		steps := parseLogSteps(logText)
		if len(steps) == 0 {
			// Distinguish a parse miss from a genuinely clean build so the user
			// knows to fall back to the full log rather than trusting silence.
			fmt.Fprintln(os.Stderr, "Could not identify build steps in this log; re-run without --failed-only for the full output.")
			return nil
		}
		failed := failedSteps(steps)
		if len(failed) == 0 {
			fmt.Println("No failed steps detected in the log.")
			return nil
		}
		fmt.Print(joinStepBodies(failed))
		return nil
	}

	fmt.Print(logText)
	return nil
}
