package cmd

import (
	"fmt"
	"strconv"

	"br/internal/api"

	"github.com/spf13/cobra"
)

func init() {
	buildCmd.AddCommand(&cobra.Command{
		Use:   "view <build-number>",
		Short: "Show details of a specific build",
		Args:  cobra.ExactArgs(1),
		Example: `  br build view 123
  br build view 123 --app my-app-slug`,
		RunE: runBuildView,
	})
}

func runBuildView(cmd *cobra.Command, args []string) error {
	buildNumber, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid build number: %s", args[0])
	}

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

	icon, statusText := statusDisplay(*build)
	fmt.Printf("%s %s  #%d  %s  (branch: %s)\n",
		icon, statusText, build.BuildNumber, build.TriggeredWorkflow, build.Branch)
	if build.CommitMessage != "" {
		hash := ""
		if len(build.CommitHash) >= 7 {
			hash = fmt.Sprintf(" (%s)", build.CommitHash[:7])
		}
		fmt.Printf("  Commit:    %s%s\n", truncate(build.CommitMessage, 72), hash)
	}
	fmt.Printf("  Triggered: %s\n", timeAgo(build.TriggeredAt))
	if build.Status != api.StatusRunning && build.Duration > 0 {
		fmt.Printf("  Duration:  %s\n", formatDuration(build.Duration))
	}

	// For finished failed builds, try to parse step failures from the log
	if build.Status == api.StatusError {
		fmt.Println()
		logText, _, err := client.FetchLog(ctx, appSlug, build.Slug)
		if err == nil && logText != "" {
			for _, s := range failedSteps(parseLogSteps(logText)) {
				fmt.Printf("  ✗ Step failed: %s (exit code: %d)\n", s.Name, s.ExitCode)
			}
		}
		fmt.Printf("\n  To see full logs:   br build logs %d\n", build.BuildNumber)
		fmt.Printf("  To see errors only: br build logs %d --failed-only\n", build.BuildNumber)
	} else if build.Status == api.StatusRunning {
		fmt.Printf("\n  Build is still running. Follow along: br build logs %d\n", build.BuildNumber)
	}
	return nil
}
