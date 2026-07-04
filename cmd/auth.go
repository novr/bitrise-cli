package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

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
	loginCmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with Bitrise using a Personal Access Token",
		RunE:  runAuthLogin,
	}
	loginCmd.Flags().Bool("no-browser", false, "Do not open the token settings page in a browser")
	loginCmd.Flags().Bool("with-token", false, "Read the token from standard input (for pipes/CI)")
	authCmd.AddCommand(loginCmd)
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

// readLoginToken obtains the PAT either from stdin (--with-token, for
// pipes/CI) or via an interactive hidden prompt. The browser is opened only
// for the interactive path on a real terminal.
func readLoginToken(cmd *cobra.Command, withToken bool) (string, error) {
	if withToken {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return "", fmt.Errorf("failed to read token from stdin: %w", err)
		}
		return strings.TrimSpace(string(data)), nil
	}

	noBrowser, _ := cmd.Flags().GetBool("no-browser")
	// Skip the browser when asked, or when stdin is not a terminal (CI, SSH)
	// where launching a browser is pointless or impossible.
	if !noBrowser && term.IsTerminal(int(os.Stdin.Fd())) {
		openPATSettingsPage()
	}
	fmt.Print("? Paste your Bitrise Personal Access Token: ")
	tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		return "", fmt.Errorf("failed to read token: %w", err)
	}
	return string(tokenBytes), nil
}

func runAuthLogin(cmd *cobra.Command, args []string) error {
	withToken, _ := cmd.Flags().GetBool("with-token")

	token, err := readLoginToken(cmd, withToken)
	if err != nil {
		return err
	}
	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}

	fmt.Print("Validating... ")
	client := api.NewClient(token)
	user, err := client.GetMe(cmd.Context())
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
	// Return errors (non-zero exit) so `br auth status && ...` is reliable in scripts.
	token, err := config.GetToken()
	if err != nil {
		return fmt.Errorf("not authenticated: run 'br auth login' or set BITRISE_TOKEN")
	}
	user, err := api.NewClient(token).GetMe(cmd.Context())
	if err != nil {
		return fmt.Errorf("token invalid: %w", err)
	}
	fmt.Printf("✓ Authenticated as %s (%s)\n", user.Username, user.Email)
	return nil
}
