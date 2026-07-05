package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/novr/bitrise-cli/internal/api"
	"github.com/novr/bitrise-cli/internal/config"

	"github.com/spf13/cobra"
)

var buildCmd = &cobra.Command{
	Use:   "build",
	Short: "View and manage builds",
}

func init() {
	buildCmd.PersistentFlags().String("app", "", "Bitrise app slug (overrides auto-detection)")
	rootCmd.AddCommand(buildCmd)
}

func newAPIClient() (*api.Client, error) {
	token, err := config.GetToken()
	if err != nil {
		return nil, err
	}
	return api.NewClient(token), nil
}

// resolveAppSlug determines the Bitrise app slug to use, in priority order:
//  1. --app flag
//  2. BITRISE_APP_SLUG environment variable
//  3. Git remote URL matched against user's Bitrise apps
//  4. default_app in config
func resolveAppSlug(ctx context.Context, cmd *cobra.Command, client *api.Client) (string, error) {
	if slug, _ := cmd.Flags().GetString("app"); slug != "" {
		return slug, nil
	}
	if slug := os.Getenv("BITRISE_APP_SLUG"); slug != "" {
		return slug, nil
	}
	slug, err := detectAppFromGit(ctx, client)
	if err == nil {
		return slug, nil
	}
	// A remote that exists but matches no app is fatal: falling back to
	// default_app here would silently target the wrong app.
	if !errors.Is(err, errNoGitRemote) {
		return "", err
	}

	cfg, cfgErr := config.Load()
	if cfgErr == nil && cfg.DefaultApp != "" {
		return cfg.DefaultApp, nil
	}
	return "", fmt.Errorf("could not determine Bitrise app\nTip: use --app <slug>, set BITRISE_APP_SLUG, or run from a git repo connected to Bitrise")
}

// errNoGitRemote means git-based detection could not run for a benign reason
// (git absent, not a repo, or no origin remote), so falling back to default_app
// is safe. Unexpected git failures (corruption, permissions) are surfaced
// instead, so they can't silently resolve to the wrong app.
var errNoGitRemote = errors.New("no git remote")

func detectAppFromGit(ctx context.Context, client *api.Client) (string, error) {
	gitCmd := exec.Command("git", "remote", "get-url", "origin")
	var stderr bytes.Buffer
	gitCmd.Stderr = &stderr
	out, err := gitCmd.Output()
	if err != nil {
		if isBenignGitError(err, stderr.String()) {
			return "", errNoGitRemote
		}
		return "", fmt.Errorf("git remote lookup failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	remoteURL := strings.TrimSpace(string(out))
	normalized := normalizeGitURL(remoteURL)

	apps, err := client.ListApps(ctx)
	if err != nil {
		return "", err
	}
	for _, app := range apps {
		if normalizeGitURL(app.RepoURL) == normalized {
			return app.Slug, nil
		}
	}
	return "", fmt.Errorf("git remote %s is not connected to any Bitrise app you can access; use --app <slug> to override", remoteURL)
}

// isBenignGitError reports whether a git failure just means "no remote to
// detect from" (git missing, not a repo, or origin unset) rather than a real
// problem worth surfacing.
func isBenignGitError(err error, stderr string) bool {
	if errors.Is(err, exec.ErrNotFound) {
		return true // git not installed
	}
	s := strings.ToLower(stderr)
	// exit 2 = "no such remote" (origin unset): unambiguously benign, and
	// locale-independent.
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
		return true
	}
	// 128 is overloaded (not-a-repo, but also permission/corruption), so require
	// stderr to confirm a benign cause rather than blanket-trusting the code.
	return strings.Contains(s, "not a git repository") ||
		strings.Contains(s, "no such remote") ||
		strings.Contains(s, "no such file")
}

func normalizeGitURL(rawURL string) string {
	u := strings.TrimSuffix(strings.TrimSpace(rawURL), ".git")
	u = strings.ToLower(u)
	if strings.HasPrefix(u, "git@") {
		u = strings.TrimPrefix(u, "git@")
		u = strings.Replace(u, ":", "/", 1)
	} else if parsed, err := url.Parse(u); err == nil && parsed.Host != "" {
		u = parsed.Host + parsed.Path
	}
	return strings.TrimPrefix(u, "www.")
}

// statusDisplay returns an icon plus a label. The label comes from the API's
// status_text (authoritative) so it never drifts from the numeric code; the
// icon is derived from the numeric status.
func statusDisplay(b api.Build) (icon, text string) {
	switch b.Status {
	case api.StatusSuccess:
		icon = "✓"
	case api.StatusError:
		icon = "✗"
	case api.StatusAborted:
		icon = "−"
	default:
		icon = "⟳"
	}
	text = b.StatusText
	if text == "" {
		if b.Status == api.StatusRunning {
			text = "running"
		} else {
			text = "unknown"
		}
	}
	return icon, text
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func elapsed(t time.Time) string {
	d := time.Since(t)
	if d < time.Minute {
		return fmt.Sprintf("%ds elapsed", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm elapsed", int(d.Minutes()))
}

func formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%dm%ds", seconds/60, seconds%60)
	}
	return fmt.Sprintf("%dh%dm", seconds/3600, (seconds%3600)/60)
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n-1]) + "…"
}
