package cmd

import (
	"fmt"

	"br/internal/api"

	"github.com/spf13/cobra"
)

var appCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage Bitrise apps",
}

func init() {
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List your Bitrise apps",
		RunE:  runAppList,
	}
	listCmd.Flags().String("json", "", "Output JSON: comma-separated fields (e.g. slug,title) or 'all'")
	appCmd.AddCommand(listCmd)
	rootCmd.AddCommand(appCmd)
}

func appToFieldMap(a api.App) map[string]interface{} {
	return map[string]interface{}{
		"slug":    a.Slug,
		"title":   a.Title,
		"repoURL": a.RepoURL,
	}
}

func runAppList(cmd *cobra.Command, args []string) error {
	jsonFields, _ := cmd.Flags().GetString("json")
	requested, err := parseJSONFields(jsonFields, sortedKeys(appToFieldMap(api.App{})))
	if err != nil {
		return err
	}

	client, err := newAPIClient()
	if err != nil {
		return err
	}
	apps, err := client.ListApps(cmd.Context())
	if err != nil {
		return err
	}

	if jsonFields != "" {
		rows := make([]map[string]interface{}, 0, len(apps))
		for _, a := range apps {
			rows = append(rows, appToFieldMap(a))
		}
		return printJSON(rows, requested)
	}

	if len(apps) == 0 {
		fmt.Println("No apps found.")
		return nil
	}
	fmt.Printf("%-40s %-36s %s\n", "Title", "Slug", "Repo")
	fmt.Printf("%-40s %-36s %s\n", "-----", "----", "----")
	for _, a := range apps {
		fmt.Printf("%-40s %-36s %s\n", truncate(a.Title, 40), a.Slug, a.RepoURL)
	}
	return nil
}
