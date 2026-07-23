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

// resolveAppSlug picks an app from explicit sources only (global default_app was
// removed to prevent silently targeting the wrong app in multi-repo setups).
func resolveAppSlug(ctx context.Context, cmd *cobra.Command, client *api.Client) (string, error) {
	res, err := resolveAppSlugDetailed(ctx, cmd, client, false)
	return res.Slug, err
}

type appResolution struct {
	Slug      string
	LocalPath string
	GitSlug   string
	GitErr    error
}

// probeGit forces git detection even when .br.yml wins, so doctor can compare slugs
// without duplicating the priority chain.
func resolveAppSlugDetailed(ctx context.Context, cmd *cobra.Command, client *api.Client, probeGit bool) (appResolution, error) {
	var res appResolution

	if slug, _ := cmd.Flags().GetString("app"); slug != "" {
		res.Slug = slug
		return res, nil
	}
	if slug := os.Getenv("BITRISE_APP_SLUG"); slug != "" {
		res.Slug = slug
		return res, nil
	}

	local, path, err := config.FindLocalConfig()
	if err != nil {
		return res, err
	}

	if probeGit && client != nil {
		res.GitSlug, res.GitErr = detectAppFromGit(ctx, client)
	}

	if local != nil && local.App != "" {
		res.Slug = local.App
		res.LocalPath = path
		return res, nil
	}

	if !probeGit && client != nil {
		res.GitSlug, res.GitErr = detectAppFromGit(ctx, client)
	} else if client == nil {
		res.GitErr = errNoGitRemote
	}

	if res.GitErr == nil {
		res.Slug = res.GitSlug
		return res, nil
	}
	if !errors.Is(res.GitErr, errNoGitRemote) {
		return res, res.GitErr
	}
	return res, fmt.Errorf("could not determine Bitrise app\nTip: use --app <slug>, set BITRISE_APP_SLUG, run br config set app <slug>, or run from a git repo connected to Bitrise")
}

// errNoGitRemote distinguishes "nothing to detect from" from real git failures;
// only the former may fall through to "could not determine app".
var errNoGitRemote = errors.New("no git remote")

func detectAppFromGit(ctx context.Context, client *api.Client) (string, error) {
	gitCmd := exec.CommandContext(ctx, "git", "remote", "get-url", "origin")
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

const branchCurrent = "@current"

// resolveBranchFilter expands @current to the git HEAD branch name.
func resolveBranchFilter(ctx context.Context, branch string) (string, error) {
	branch = strings.TrimSpace(branch)
	if branch != branchCurrent {
		return branch, nil
	}
	return currentGitBranch(ctx)
}

func currentGitBranch(ctx context.Context) (string, error) {
	gitCmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	var stderr bytes.Buffer
	gitCmd.Stderr = &stderr
	out, err := gitCmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("git not found in PATH: cannot resolve %s", branchCurrent)
		}
		if isBenignGitError(err, stderr.String()) {
			return "", fmt.Errorf("not a git repository: cannot resolve %s", branchCurrent)
		}
		return "", fmt.Errorf("git branch lookup failed: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	name := strings.TrimSpace(string(out))
	if name == "" || name == "HEAD" {
		return "", fmt.Errorf("detached HEAD: cannot resolve %s", branchCurrent)
	}
	return name, nil
}

func isBenignGitError(err error, stderr string) bool {
	if errors.Is(err, exec.ErrNotFound) {
		return true
	}
	s := strings.ToLower(stderr)
	// exit 2: "no such remote" — locale-independent, unlike parsing stderr text.
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 2 {
		return true
	}
	// exit 128 covers both "not a repo" and permission errors; require stderr proof.
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

// statusDisplay uses status_text for the label (API-authoritative) and derives
// the icon from the numeric code so the two never drift.
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
