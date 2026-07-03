package cmd

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

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
	appSlug, err := resolveAppSlug(cmd.Parent(), client)
	if err != nil {
		return err
	}

	build, err := client.GetBuildByNumber(appSlug, buildNumber)
	if err != nil {
		return err
	}

	logText, archived, err := client.FetchLog(build.Slug)
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
	if !archived && build.Status == 0 {
		fmt.Printf("# Build #%d is still running — showing partial log\n\n", build.BuildNumber)
	}

	if failedOnly {
		filtered := extractFailedStepSections(logText)
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

var (
	stepSectionStartRe = regexp.MustCompile(`(?m)^\s*\|\s*\(\d+\)`)
	stepExitFailRe     = regexp.MustCompile(`(?i)exit.?code[:=\s]+([1-9]\d*)`)
)

// extractFailedStepSections splits the log into step sections (delimited by step
// header lines) and returns only the sections where a non-zero exit code was found.
func extractFailedStepSections(logText string) string {
	// Split on step header boundaries: lines containing "| (N)"
	// We scan line-by-line to keep the boundary line with the section.
	lines := strings.Split(logText, "\n")

	type section struct {
		lines []string
	}

	var sections []section
	var current []string

	for _, line := range lines {
		if stepSectionStartRe.MatchString(line) && len(current) > 0 {
			sections = append(sections, section{lines: current})
			current = nil
		}
		current = append(current, line)
	}
	if len(current) > 0 {
		sections = append(sections, section{lines: current})
	}

	var sb strings.Builder
	for _, sec := range sections {
		text := strings.Join(sec.lines, "\n")
		if stepExitFailRe.MatchString(text) {
			sb.WriteString(text)
			if !strings.HasSuffix(text, "\n") {
				sb.WriteByte('\n')
			}
		}
	}
	return sb.String()
}
