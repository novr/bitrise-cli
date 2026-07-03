package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"br/internal/api"

	"github.com/spf13/cobra"
)

func init() {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List recent builds",
		Example: `  br build list
  br build list --limit 20 --branch main
  br build list --json status,buildNumber,branch,workflow
  br build list --status failed`,
		RunE: runBuildList,
	}
	cmd.Flags().IntP("limit", "n", 10, "Number of builds to show")
	cmd.Flags().String("branch", "", "Filter by branch name")
	cmd.Flags().String("workflow", "", "Filter by workflow name")
	cmd.Flags().String("status", "", "Filter by status: success, failed, running, aborted")
	cmd.Flags().String("json", "", "Output JSON; optionally specify comma-separated fields (e.g. status,buildNumber)")
	cmd.Flags().Lookup("json").NoOptDefVal = "*"
	buildCmd.AddCommand(cmd)
}

func runBuildList(cmd *cobra.Command, args []string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	branch, _ := cmd.Flags().GetString("branch")
	workflow, _ := cmd.Flags().GetString("workflow")
	statusFilter, _ := cmd.Flags().GetString("status")
	jsonFields, _ := cmd.Flags().GetString("json")

	// Validate input before any auth/network work.
	statusCode, err := statusNameToCode(statusFilter)
	if err != nil {
		return err
	}

	client, err := newAPIClient()
	if err != nil {
		return err
	}
	appSlug, err := resolveAppSlug(cmd.Parent(), client)
	if err != nil {
		return err
	}

	builds, err := client.ListBuilds(appSlug, api.ListBuildsParams{
		Limit:    limit,
		Branch:   branch,
		Workflow: workflow,
		Status:   statusCode,
	})
	if err != nil {
		return err
	}

	if jsonFields != "" {
		return printBuildsJSON(builds, jsonFields)
	}
	return printBuildsTable(builds, appSlug)
}

func statusNameToCode(name string) (string, error) {
	switch strings.ToLower(name) {
	case "":
		return "", nil
	case "success":
		return "1", nil
	case "failed", "failure":
		return "2", nil
	case "running", "in-progress":
		return "0", nil
	case "aborted":
		return "4", nil
	default:
		return "", fmt.Errorf("invalid --status %q (valid: success, failed, running, aborted)", name)
	}
}

func printBuildsTable(builds []api.Build, appSlug string) error {
	if len(builds) == 0 {
		fmt.Println("No builds found.")
		return nil
	}
	fmt.Printf("Showing %d build(s)  app: %s\n\n", len(builds), appSlug)
	for _, b := range builds {
		icon, statusText := statusIcon(b.Status)
		timeStr := ""
		if b.Status == 0 {
			timeStr = elapsed(b.TriggeredAt)
		} else if b.FinishedAt != nil {
			timeStr = timeAgo(*b.FinishedAt)
		}
		commit := truncate(b.CommitMessage, 28)
		fmt.Printf("%s %-8s #%-5d %-22s [%-28s] (workflow: %-16s) %s\n",
			icon, statusText,
			b.BuildNumber,
			truncate(b.Branch, 22),
			commit,
			b.TriggeredWorkflow,
			timeStr,
		)
	}
	return nil
}

func printBuildsJSON(builds []api.Build, fields string) error {
	requested := map[string]bool{}
	if fields != "*" && fields != "" {
		for _, f := range strings.Split(fields, ",") {
			requested[strings.TrimSpace(f)] = true
		}
	}

	result := make([]map[string]interface{}, 0, len(builds))
	for _, b := range builds {
		all := map[string]interface{}{
			"status":          b.StatusText,
			"buildNumber":     b.BuildNumber,
			"branch":          b.Branch,
			"workflow":        b.TriggeredWorkflow,
			"slug":            b.Slug,
			"commitMessage":   b.CommitMessage,
			"commitHash":      b.CommitHash,
			"triggeredAt":     b.TriggeredAt,
			"finishedAt":      b.FinishedAt,
			"durationSeconds": b.Duration,
		}
		if len(requested) == 0 {
			result = append(result, all)
		} else {
			row := map[string]interface{}{}
			for k, v := range all {
				if requested[k] {
					row[k] = v
				}
			}
			result = append(result, row)
		}
	}

	out, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(out))
	return nil
}
