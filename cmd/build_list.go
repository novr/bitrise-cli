package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
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
  br build list --json all
  br build list --status failed`,
		RunE: runBuildList,
	}
	cmd.Flags().IntP("limit", "n", 10, "Number of builds to show")
	cmd.Flags().String("branch", "", "Filter by branch name")
	cmd.Flags().String("workflow", "", "Filter by workflow name")
	cmd.Flags().String("status", "", "Filter by status: success, failed, running, aborted")
	cmd.Flags().String("json", "", "Output JSON: comma-separated fields (e.g. status,buildNumber) or 'all'")
	buildCmd.AddCommand(cmd)
}

func runBuildList(cmd *cobra.Command, args []string) error {
	limit, _ := cmd.Flags().GetInt("limit")
	branch, _ := cmd.Flags().GetString("branch")
	workflow, _ := cmd.Flags().GetString("workflow")
	statusFilter, _ := cmd.Flags().GetString("status")
	jsonFields, _ := cmd.Flags().GetString("json")

	// Validate input before any auth/network work.
	status, err := parseStatusFilter(statusFilter)
	if err != nil {
		return err
	}
	requestedFields, err := parseJSONFields(jsonFields)
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
		Status:   status,
	})
	if err != nil {
		return err
	}

	if jsonFields != "" {
		return printBuildsJSON(builds, requestedFields)
	}
	return printBuildsTable(builds, appSlug)
}

// parseStatusFilter maps a user-facing status name to a build status.
// An empty name means no filter (nil).
func parseStatusFilter(name string) (*api.BuildStatus, error) {
	var s api.BuildStatus
	switch strings.ToLower(name) {
	case "":
		return nil, nil
	case "success":
		s = api.StatusSuccess
	case "failed", "failure":
		s = api.StatusFailed
	case "running", "in-progress":
		s = api.StatusRunning
	case "aborted":
		s = api.StatusAborted
	default:
		return nil, fmt.Errorf("invalid --status %q (valid: success, failed, running, aborted)", name)
	}
	return &s, nil
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
		if b.Status == api.StatusRunning {
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

func buildToFieldMap(b api.Build) map[string]interface{} {
	return map[string]interface{}{
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
}

func validBuildFields() []string {
	keys := make([]string, 0)
	for k := range buildToFieldMap(api.Build{}) {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// parseJSONFields validates a comma-separated --json field list against the
// known build fields. An empty string, "*" or "all" means "all fields" (nil map).
func parseJSONFields(fields string) (map[string]bool, error) {
	if fields == "" || fields == "*" || fields == "all" {
		return nil, nil
	}
	valid := validBuildFields()
	validSet := map[string]bool{}
	for _, k := range valid {
		validSet[k] = true
	}
	requested := map[string]bool{}
	for _, f := range strings.Split(fields, ",") {
		name := strings.TrimSpace(f)
		if name == "" {
			continue
		}
		if !validSet[name] {
			return nil, fmt.Errorf("unknown --json field %q (valid: %s)", name, strings.Join(valid, ", "))
		}
		requested[name] = true
	}
	return requested, nil
}

func printBuildsJSON(builds []api.Build, requested map[string]bool) error {
	result := make([]map[string]interface{}, 0, len(builds))
	for _, b := range builds {
		all := buildToFieldMap(b)
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
