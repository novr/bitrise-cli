package cmd

import (
	"fmt"

	"br/internal/config"

	"github.com/spf13/cobra"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage br configuration",
}

func init() {
	configCmd.AddCommand(&cobra.Command{
		Use:   "set-default-app <app-slug>",
		Short: "Set the default app used when git detection can't identify one",
		Args:  cobra.ExactArgs(1),
		RunE:  runConfigSetDefaultApp,
	})
	configCmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE:  runConfigShow,
	})
	rootCmd.AddCommand(configCmd)
}

func runConfigSetDefaultApp(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.DefaultApp = args[0]
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Printf("✓ default app set to %s\n", args[0])
	return nil
}

func runConfigShow(cmd *cobra.Command, args []string) error {
	path, _ := config.Path()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	authed := "no"
	if cfg.Token != "" {
		authed = "yes"
	}
	fmt.Printf("Config file:  %s\n", path)
	fmt.Printf("Authenticated: %s\n", authed)
	fmt.Printf("Default app:   %s\n", orNone(cfg.DefaultApp))
	return nil
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
