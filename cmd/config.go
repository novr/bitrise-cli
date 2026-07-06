package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/novr/bitrise-cli/internal/api"
	"github.com/novr/bitrise-cli/internal/config"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage br configuration",
}

var configSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Set configuration values",
}

func init() {
	configSetCmd.AddCommand(&cobra.Command{
		Use:   "app <app-slug>",
		Short: "Write app slug to .br.yml in the current directory",
		Args:  cobra.ExactArgs(1),
		RunE:  runConfigSetApp,
	})
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE:  runConfigShow,
	})
	rootCmd.AddCommand(configCmd)
}

func runConfigSetApp(cmd *cobra.Command, args []string) error {
	slug := strings.TrimSpace(args[0])
	if slug == "" {
		return fmt.Errorf("app slug must not be empty")
	}

	validated := false
	if client, err := newAPIClient(); err == nil {
		if _, err := client.ListBuilds(cmd.Context(), slug, api.ListBuildsParams{Limit: 1}); err != nil {
			return fmt.Errorf("app slug %q not found or not accessible: %w", slug, err)
		}
		validated = true
	} else {
		fmt.Fprintf(cmd.ErrOrStderr(), "⚠ warning: not authenticated; writing %q without API validation\n", slug)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	path, err := config.WriteLocalConfig(cwd, slug)
	if err != nil {
		return err
	}
	if validated {
		fmt.Printf("✓ wrote app to %s\n", path)
	} else {
		fmt.Printf("✓ wrote app to %s (unvalidated)\n", path)
	}
	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	path, _ := config.Path()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	authed := "no"
	source := ""
	if _, err := config.GetToken(); err == nil {
		authed = "yes"
		if cfg.Token == "" {
			source = " (from environment)"
		}
	}
	fmt.Printf("Config file:   %s\n", path)
	fmt.Printf("Authenticated: %s%s\n", authed, source)

	local, localPath, err := config.FindLocalConfig()
	if err != nil {
		return err
	}
	if local != nil && local.App != "" {
		fmt.Printf("Local config:  %s\n", localPath)
		fmt.Printf("App:           %s\n", local.App)
	} else {
		fmt.Printf("Local config:  (none)\n")
		fmt.Printf("App:           (none)\n")
	}
	return nil
}

func localConfigCwdPath() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Join(cwd, config.LocalConfigFileName), nil
}
