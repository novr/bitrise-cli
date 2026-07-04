package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var appCmd = &cobra.Command{
	Use:   "app",
	Short: "Manage Bitrise apps",
}

func init() {
	appCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List your Bitrise apps",
		RunE:  runAppList,
	})
	rootCmd.AddCommand(appCmd)
}

func runAppList(cmd *cobra.Command, args []string) error {
	client, err := newAPIClient()
	if err != nil {
		return err
	}
	apps, err := client.ListApps(cmd.Context())
	if err != nil {
		return err
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
