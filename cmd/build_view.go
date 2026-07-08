package cmd

import (
	"context"
	"fmt"
	"strconv"

	"github.com/novr/bitrise-cli/internal/api"

	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "view <build-number>",
		Short: "Show details of a specific build",
		Args:  cobra.ExactArgs(1),
		Example: `  br build view 123
  br build view 123 --app my-app-slug
  br build view 123 --json status,buildNumber,failedSteps
  br build view 123 --json all`,
		RunE: runBuildView,
	}
	cmd.Flags().String("json", "", "Output JSON: comma-separated fields (e.g. status,buildNumber,failedSteps) or 'all'")
	buildCmd.AddCommand(cmd)
}

func runBuildView(cmd *cobra.Command, args []string) error {
	buildNumber, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid build number: %s", args[0])
	}
	jsonFields, _ := cmd.Flags().GetString("json")
	requestedFields, err := parseJSONFields(jsonFields, validBuildViewFields())
	if err != nil {
		return err
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

	if jsonFields != "" {
		failed := failedStepsForView(ctx, client, appSlug, build, requestedFields)
		return printJSONObject(buildViewToFieldMap(*build, failed), requestedFields)
	}

	return printBuildViewHuman(ctx, client, appSlug, build)
}

func buildViewToFieldMap(b api.Build, failed []logStep) map[string]interface{} {
	m := buildToFieldMap(b)
	steps := make([]map[string]interface{}, 0, len(failed))
	for _, s := range failed {
		steps = append(steps, map[string]interface{}{
			"name":     s.Name,
			"exitCode": s.ExitCode,
		})
	}
	m["failedSteps"] = steps
	return m
}

func validBuildViewFields() []string {
	return sortedKeys(buildViewToFieldMap(api.Build{}, nil))
}

func needsFailedStepLog(requested map[string]bool) bool {
	if requested == nil {
		return true
	}
	return requested["failedSteps"]
}

func failedStepsForView(ctx context.Context, client *api.Client, appSlug string, build *api.Build, requested map[string]bool) []logStep {
	if build.Status != api.StatusError || !needsFailedStepLog(requested) {
		return nil
	}
	logText, _, err := client.FetchLog(ctx, appSlug, build.Slug)
	if err != nil || logText == "" {
		return nil
	}
	return failedSteps(parseLogSteps(logText))
}

func printBuildViewHuman(ctx context.Context, client *api.Client, appSlug string, build *api.Build) error {
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
	if d := build.DurationSeconds(); d > 0 {
		fmt.Printf("  Duration:  %s\n", formatDuration(d))
	}

	if build.Status == api.StatusError {
		fmt.Println()
		for _, s := range failedStepsForView(ctx, client, appSlug, build, map[string]bool{"failedSteps": true}) {
			fmt.Printf("  ✗ Step failed: %s (exit code: %d)\n", s.Name, s.ExitCode)
		}
		fmt.Printf("\n  To see full logs:   br build logs %d\n", build.BuildNumber)
		fmt.Printf("  To see errors only: br build logs %d --failed-only\n", build.BuildNumber)
	} else if build.Status == api.StatusRunning {
		fmt.Printf("\n  Build is still running. Follow along: br build logs %d\n", build.BuildNumber)
	}
	return nil
}
