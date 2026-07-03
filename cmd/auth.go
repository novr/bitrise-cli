package cmd

import (
	"fmt"
	"os"

	"br/internal/api"
	"br/internal/config"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage authentication",
}

func init() {
	authCmd.AddCommand(&cobra.Command{
		Use:   "login",
		Short: "Authenticate with Bitrise using a Personal Access Token",
		RunE:  runAuthLogin,
	})
	authCmd.AddCommand(&cobra.Command{
		Use:   "logout",
		Short: "Remove stored credentials",
		RunE:  runAuthLogout,
	})
	authCmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show current authentication status",
		RunE:  runAuthStatus,
	})
	rootCmd.AddCommand(authCmd)
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	fmt.Print("? Paste your Bitrise Personal Access Token: ")
	tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return fmt.Errorf("failed to read token: %w", err)
	}
	token := string(tokenBytes)
	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}

	fmt.Print("Validating... ")
	client := api.NewClient(token)
	user, err := client.GetMe()
	if err != nil {
		fmt.Println("✗")
		return err
	}
	fmt.Println("✓")

	cfg, err := config.Load()
	if err != nil {
		cfg = &config.Config{}
	}
	cfg.Token = token
	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Printf("✓ Logged in as %s (%s)\n", user.Username, user.Email)
	return nil
}

func runAuthLogout(cmd *cobra.Command, args []string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Token = ""
	if err := config.Save(cfg); err != nil {
		return err
	}
	fmt.Println("✓ Logged out")
	return nil
}

func runAuthStatus(cmd *cobra.Command, args []string) error {
	token, err := config.GetToken()
	if err != nil {
		fmt.Println("✗ Not authenticated")
		fmt.Println("  Run 'br auth login' or set BITRISE_TOKEN")
		return nil
	}
	client := api.NewClient(token)
	user, err := client.GetMe()
	if err != nil {
		fmt.Printf("✗ Token invalid: %v\n", err)
		return nil
	}
	fmt.Printf("✓ Authenticated as %s (%s)\n", user.Username, user.Email)
	return nil
}
