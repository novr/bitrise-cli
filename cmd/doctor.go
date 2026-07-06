package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/novr/bitrise-cli/internal/api"
	"github.com/novr/bitrise-cli/internal/config"

	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnose authentication and app resolution",
	RunE:  runDoctor,
}

func init() {
	doctorCmd.PersistentFlags().String("app", "", "Bitrise app slug (overrides auto-detection)")
	rootCmd.AddCommand(doctorCmd)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	issues := 0

	token, authErr := config.GetToken()
	if authErr != nil {
		fmt.Println("✗ authentication: not authenticated")
		issues++
	} else {
		client := api.NewClient(token)
		if err := client.Verify(cmd.Context()); err != nil {
			fmt.Printf("✗ authentication: token invalid: %v\n", err)
			issues++
		} else {
			fmt.Println("✓ authentication: ok")
		}
	}

	if deprecated, err := config.DeprecatedDefaultApp(); err != nil {
		return err
	} else if deprecated {
		fmt.Println("⚠ warning: config.yml contains deprecated default_app (ignored); remove it")
	}

	var client *api.Client
	if authErr == nil {
		client = api.NewClient(token)
	}

	slug, localPath, gitSlug, gitErr, resolveErr := diagnoseAppResolution(cmd.Context(), cmd, client)
	if resolveErr != nil {
		fmt.Printf("✗ app resolution: %v\n", resolveErr)
		issues++
	} else {
		fmt.Printf("✓ app resolution: %s\n", slug)
	}

	if localPath != "" {
		fmt.Printf("  local config: %s\n", localPath)
		if cwdPath, err := localConfigCwdPath(); err == nil && localPath != cwdPath {
			fmt.Printf("  info: using .br.yml from %s\n", localPath)
		}
	}

	if slug != "" && client != nil {
		if _, err := client.ListBuilds(cmd.Context(), slug, api.ListBuildsParams{Limit: 1}); err != nil {
			fmt.Printf("✗ API reachability: app %q not accessible: %v\n", slug, err)
			issues++
		} else {
			fmt.Println("✓ API reachability: ok")
		}
	}

	if localPath != "" && gitErr == nil && gitSlug != "" && gitSlug != slug {
		fmt.Printf("⚠ warning: .br.yml app (%s) differs from git-detected app (%s)\n", slug, gitSlug)
	}
	if localPath != "" && gitErr != nil && !errors.Is(gitErr, errNoGitRemote) {
		fmt.Printf("  info: .br.yml supplies app; git detection failed (%v)\n", gitErr)
	}

	if issues > 0 {
		return fmt.Errorf("%d issue(s) found", issues)
	}
	return nil
}

func diagnoseAppResolution(ctx context.Context, cmd *cobra.Command, client *api.Client) (slug, localPath, gitSlug string, gitErr, resolveErr error) {
	if s, _ := cmd.Flags().GetString("app"); s != "" {
		return s, "", "", nil, nil
	}
	if s := os.Getenv("BITRISE_APP_SLUG"); s != "" {
		return s, "", "", nil, nil
	}

	local, path, findErr := config.FindLocalConfig()
	if findErr != nil {
		return "", "", "", nil, findErr
	}

	if client != nil {
		gitSlug, gitErr = detectAppFromGit(ctx, client)
	} else {
		gitErr = errNoGitRemote
	}

	if local != nil && local.App != "" {
		return local.App, path, gitSlug, gitErr, nil
	}

	if gitErr == nil {
		return gitSlug, "", gitSlug, nil, nil
	}
	if !errors.Is(gitErr, errNoGitRemote) {
		return "", "", "", gitErr, gitErr
	}
	return "", "", "", gitErr, fmt.Errorf("could not determine Bitrise app")
}
