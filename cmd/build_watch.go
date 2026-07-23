package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"time"

	"github.com/novr/bitrise-cli/internal/api"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const minWatchInterval = 3 * time.Second

var getBuildForWatch = func(ctx context.Context, client *api.Client, appSlug string, buildNumber int) (*api.Build, error) {
	return client.GetBuildByNumber(ctx, appSlug, buildNumber)
}

var (
	newAPIClientForWatch   = newAPIClient
	resolveAppSlugForWatch = resolveAppSlug
)

func init() {
	cmd := &cobra.Command{
		Use:   "watch <build-number>",
		Short: "Watch a build until it finishes",
		Args:  cobra.ExactArgs(1),
		Example: `  br build watch 123
  br build watch 123 --exit-status
  br build watch 123 --json status,buildNumber,failedSteps`,
		RunE: runBuildWatch,
	}
	cmd.Flags().Duration("interval", 5*time.Second, "Polling interval while the build is running")
	cmd.Flags().Bool("exit-status", false, "Exit with code 1 when the build fails or is aborted")
	cmd.Flags().String("json", "", "Output JSON on completion: comma-separated fields (e.g. status,buildNumber,failedSteps) or 'all'")
	buildCmd.AddCommand(cmd)
}

func runBuildWatch(cmd *cobra.Command, args []string) error {
	buildNumber, err := strconv.Atoi(args[0])
	if err != nil {
		return fmt.Errorf("invalid build number: %s", args[0])
	}
	interval, _ := cmd.Flags().GetDuration("interval")
	if err := validateWatchInterval(interval); err != nil {
		return err
	}
	exitStatus, _ := cmd.Flags().GetBool("exit-status")
	jsonFields, _ := cmd.Flags().GetString("json")
	requestedFields, err := parseJSONFields(jsonFields, validBuildViewFields())
	if err != nil {
		return err
	}

	client, err := newAPIClientForWatch()
	if err != nil {
		return err
	}
	ctx := cmd.Context()
	appSlug, err := resolveAppSlugForWatch(ctx, cmd.Parent(), client)
	if err != nil {
		return err
	}

	isTTY := isWriterTerminal(cmd.OutOrStdout()) && jsonFields == ""
	watchOut := cmd.OutOrStdout()
	if jsonFields != "" {
		watchOut = io.Discard
	}
	build, err := watchBuild(ctx, client, appSlug, buildNumber, interval, watchOut, isTTY)
	if err != nil {
		return err
	}

	if jsonFields != "" {
		failed := failedStepsForView(ctx, client, appSlug, build, requestedFields)
		return printJSONObject(buildViewToFieldMap(*build, failed), requestedFields)
	}

	if isTTY {
		fmt.Fprintln(cmd.OutOrStdout())
	}
	if build.Status == api.StatusError {
		for _, s := range failedStepsForView(ctx, client, appSlug, build, map[string]bool{fieldFailedSteps: true}) {
			fmt.Fprintf(cmd.OutOrStdout(), "  ✗ Step failed: %s (exit code: %d)\n", s.Name, s.ExitCode)
		}
	}

	return buildWatchExitError(exitStatus, *build)
}

func buildWatchExitError(exitStatus bool, build api.Build) error {
	if exitStatus && build.Status != api.StatusSuccess {
		_, statusText := statusDisplay(build)
		return fmt.Errorf("build #%d finished with status %s", build.BuildNumber, statusText)
	}
	return nil
}

func validateWatchInterval(interval time.Duration) error {
	if interval < minWatchInterval {
		return fmt.Errorf("invalid --interval %s (minimum %s)", interval, minWatchInterval)
	}
	return nil
}

func isWriterTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	return term.IsTerminal(int(f.Fd()))
}

func watchBuild(ctx context.Context, client *api.Client, appSlug string, buildNumber int, interval time.Duration, out io.Writer, isTTY bool) (*api.Build, error) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		build, err := getBuildForWatch(ctx, client, appSlug, buildNumber)
		if err != nil {
			return nil, err
		}
		printWatchStatus(out, isTTY, build)
		if build.Status != api.StatusRunning {
			return build, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func printWatchStatus(out io.Writer, isTTY bool, build *api.Build) {
	icon, statusText := statusDisplay(*build)
	timeStr := ""
	if build.Status == api.StatusRunning {
		timeStr = elapsed(build.TriggeredAt)
	} else if build.FinishedAt != nil {
		timeStr = timeAgo(*build.FinishedAt)
	}
	line := fmt.Sprintf("%s %s  #%d  %s  (branch: %s)  %s",
		icon, statusText, build.BuildNumber, build.TriggeredWorkflow, build.Branch, timeStr)
	if isTTY {
		fmt.Fprintf(out, "\r\033[K%s", line)
	} else {
		fmt.Fprintln(out, line)
	}
}
