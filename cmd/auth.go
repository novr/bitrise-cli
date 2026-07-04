package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"br/internal/api"
	"br/internal/config"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

const patSettingsURL = "https://app.bitrise.io/me/profile#/security"

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

	// Show the URL rather than opening a browser (auto-opening is intrusive).
	fmt.Printf("Create a Personal Access Token at %s\n", patSettingsURL)
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
	if err := client.Verify(cmd.Context()); err != nil {
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

	fmt.Printf("✓ %s\n", withIdentity("Logged in", cmd.Context(), client))
	return nil
}

// withIdentity appends "as <user> (<email>)" to verb when the token resolves to
// a user via /me. Workspace API tokens have no /me access, so verb is returned
// as-is (e.g. "Logged in").
func withIdentity(verb string, ctx context.Context, client *api.Client) string {
	if u, err := client.GetMe(ctx); err == nil && u.Username != "" {
		return fmt.Sprintf("%s as %s (%s)", verb, u.Username, u.Email)
	}
	return verb
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
		return fmt.Errorf("not authenticated: run 'br auth login' or set BITRISE_API_TOKEN")
	}
	client := api.NewClient(token)
	if err := client.Verify(cmd.Context()); err != nil {
		return fmt.Errorf("token invalid: %w", err)
	}
	fmt.Printf("✓ %s\n", withIdentity("Authenticated", cmd.Context(), client))
	return nil
}
