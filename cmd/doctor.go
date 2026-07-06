package cmd

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

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

var newDoctorAPIClient = func(token string) *api.Client {
	return api.NewClient(token)
}

func runDoctor(cmd *cobra.Command, args []string) error {
	issues := 0
	var client *api.Client

	token, authErr := config.GetToken()
	if authErr != nil {
		fmt.Println("✗ authentication: not authenticated")
		issues++
	} else {
		client = newDoctorAPIClient(token)
		if err := client.Verify(cmd.Context()); err != nil {
			fmt.Printf("✗ authentication: token invalid: %v\n", err)
			issues++
			client = nil
		} else {
			fmt.Println("✓ authentication: ok")
		}
	}

	if slug, ok, err := config.DeprecatedDefaultAppValue(); err != nil {
		return err
	} else if ok {
		fmt.Println("⚠ warning: config.yml contains deprecated default_app (ignored); remove it")
		fmt.Printf("  tip: run br config set app %s in your project directory\n", slug)
	}

	res, resolveErr := resolveAppSlugDetailed(cmd.Context(), cmd, client, true)
	if resolveErr != nil {
		fmt.Printf("✗ app resolution: %v\n", resolveErr)
		issues++
	} else {
		fmt.Printf("✓ app resolution: %s\n", res.Slug)
	}

	if res.LocalPath != "" {
		fmt.Printf("  local config: %s\n", res.LocalPath)
		if cwdPath, err := localConfigCwdPath(); err == nil && !samePath(res.LocalPath, cwdPath) {
			fmt.Printf("  info: using .br.yml from %s\n", res.LocalPath)
		}
	}

	if res.Slug != "" && client != nil {
		if _, err := client.ListBuilds(cmd.Context(), res.Slug, api.ListBuildsParams{Limit: 1}); err != nil {
			fmt.Printf("✗ API reachability: app %q not accessible: %v\n", res.Slug, err)
			issues++
		} else {
			fmt.Println("✓ API reachability: ok")
		}
	}

	doctorSlugWarnings(res, cmd.OutOrStdout())

	if issues > 0 {
		return fmt.Errorf("%d issue(s) found", issues)
	}
	return nil
}

func doctorSlugWarnings(res appResolution, w io.Writer) {
	if res.LocalPath != "" && res.GitErr == nil && res.GitSlug != "" && res.GitSlug != res.Slug {
		fmt.Fprintf(w, "⚠ warning: .br.yml app (%s) differs from git-detected app (%s)\n", res.Slug, res.GitSlug)
	}
	if res.LocalPath != "" && res.GitErr != nil && !errors.Is(res.GitErr, errNoGitRemote) {
		fmt.Fprintf(w, "  info: .br.yml supplies app; git detection failed (%v)\n", res.GitErr)
	}
}

func samePath(a, b string) bool {
	// /var vs /private/var on macOS would otherwise show a false "inherited" path.
	a = filepath.Clean(a)
	b = filepath.Clean(b)
	if a == b {
		return true
	}
	if ra, err := filepath.EvalSymlinks(a); err == nil {
		a = ra
	}
	if rb, err := filepath.EvalSymlinks(b); err == nil {
		b = rb
	}
	return filepath.Clean(a) == filepath.Clean(b)
}
