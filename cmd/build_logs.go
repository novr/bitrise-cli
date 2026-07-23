package cmd

import (
	"fmt"
	"os"
	"strconv"

	"github.com/novr/bitrise-cli/internal/api"

	"github.com/spf13/cobra"
)

const (
	fieldSteps          = "steps"
	fieldFailedStepLogs = "failedStepLogs"
)

func init() {
	cmd := &cobra.Command{
		Use:   "logs <build-number>",
		Short: "Show build logs",
		Args:  cobra.ExactArgs(1),
		Example: `  br build logs 123
  br build logs 123 --failed-only
  br build logs 123 --json steps,failedStepLogs
  br build logs 123 --json all`,
		RunE: runBuildLogs,
	}
	cmd.Flags().Bool("failed-only", false, "Show only output from failed steps")
	cmd.Flags().String("json", "", "Output JSON: comma-separated fields (steps, failedStepLogs) or 'all'")
	buildCmd.AddCommand(cmd)
}

func runBuildLogs(cmd *cobra.Command, args []string) error {
	buildNumber, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid build number: %s", args[0])
	}
	jsonFields, _ := cmd.Flags().GetString("json")
	failedOnly, _ := cmd.Flags().GetBool("failed-only")

	if jsonFields != "" && failedOnly {
		return fmt.Errorf("--json and --failed-only are mutually exclusive")
	}
	requestedFields, err := parseJSONFields(jsonFields, validBuildLogsFields())
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

	logText, archived, err := client.FetchLog(ctx, appSlug, build.Slug)
	if err != nil {
		return fmt.Errorf("failed to fetch log: %w", err)
	}

	if jsonFields != "" {
		return printBuildLogsJSON(logText, requestedFields)
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
			fmt.Fprintln(os.Stderr, parseLogStepsFailureMessage)
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

const parseLogStepsFailureMessage = "Could not identify build steps in this log; re-run without --failed-only or --json for the full output."

func buildLogsToFieldMap(steps []logStep) map[string]interface{} {
	allSteps := make([]map[string]interface{}, 0, len(steps))
	for _, s := range steps {
		allSteps = append(allSteps, map[string]interface{}{
			"name":     s.Name,
			"exitCode": s.ExitCode,
		})
	}
	failedLogs := make([]map[string]interface{}, 0)
	for _, s := range failedSteps(steps) {
		failedLogs = append(failedLogs, map[string]interface{}{
			"name":     s.Name,
			"exitCode": s.ExitCode,
			"body":     s.Body,
		})
	}
	return map[string]interface{}{
		fieldSteps:          allSteps,
		fieldFailedStepLogs: failedLogs,
	}
}

func validBuildLogsFields() []string {
	return sortedKeys(buildLogsToFieldMap(nil))
}

func printBuildLogsJSON(logText string, requested map[string]bool) error {
	steps := parseLogSteps(logText)
	if logText != "" && len(steps) == 0 {
		fmt.Fprintln(os.Stderr, parseLogStepsFailureMessage)
	}
	return printJSONObject(buildLogsToFieldMap(steps), requested)
}
