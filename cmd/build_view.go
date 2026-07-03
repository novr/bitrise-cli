package cmd

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

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
	appSlug, err := resolveAppSlug(cmd.Parent(), client)
	if err != nil {
		return err
	}

	build, err := client.GetBuildByNumber(appSlug, buildNumber)
	if err != nil {
		return err
	}

	icon, statusText := statusIcon(build.Status)
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
	if build.Status != 0 && build.Duration > 0 {
		fmt.Printf("  Duration:  %s\n", formatDuration(build.Duration))
	}

	// For finished failed builds, try to parse step failures from the log
	if build.Status == 2 || build.Status == 3 {
		fmt.Println()
		logText, _, err := client.FetchLog(build.Slug)
		if err == nil && logText != "" {
			steps := parseStepResults(logText)
			failedSteps := filterFailed(steps)
			if len(failedSteps) > 0 {
				for _, s := range failedSteps {
					fmt.Printf("  ✗ Step failed: %s (exit code: %d)\n", s.Name, s.ExitCode)
				}
			}
		}
		fmt.Printf("\n  To see full logs:   br build logs %d\n", build.BuildNumber)
		fmt.Printf("  To see errors only: br build logs %d --failed-only\n", build.BuildNumber)
	} else if build.Status == 0 {
		fmt.Printf("\n  Build is still running. Follow along: br build logs %d\n", build.BuildNumber)
	}
	return nil
}

type stepResult struct {
	Name     string
	ExitCode int
}

var (
	stepHeaderRe = regexp.MustCompile(`\|\s*\(\d+\)\s+(.+?)(?:\s*\|)?\s*$`)
	exitCodeRe   = regexp.MustCompile(`(?i)exit.?code[:=\s]+(\d+)`)
)

func parseStepResults(logText string) []stepResult {
	var results []stepResult
	var current *stepResult

	for _, line := range strings.Split(logText, "\n") {
		if m := stepHeaderRe.FindStringSubmatch(line); m != nil {
			if current != nil {
				results = append(results, *current)
			}
			name := strings.TrimSpace(m[1])
			current = &stepResult{Name: name}
			continue
		}
		if current != nil {
			if m := exitCodeRe.FindStringSubmatch(line); m != nil {
				code, _ := strconv.Atoi(m[1])
				current.ExitCode = code
			}
		}
	}
	if current != nil {
		results = append(results, *current)
	}
	return results
}

func filterFailed(steps []stepResult) []stepResult {
	var failed []stepResult
	for _, s := range steps {
		if s.ExitCode != 0 {
			failed = append(failed, s)
		}
	}
	return failed
}
