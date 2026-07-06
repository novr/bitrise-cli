package cmd

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/novr/bitrise-cli/internal/api"
	"github.com/novr/bitrise-cli/internal/config"
	"github.com/spf13/cobra"
)

// Outside a git repo, detection must return errNoGitRemote when no .br.yml is set.
func TestDetectAppFromGitNoRemote(t *testing.T) {
	orig, _ := os.Getwd()
	t.Cleanup(func() { os.Chdir(orig) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	_, err := detectAppFromGit(context.Background(), nil)
	if !errors.Is(err, errNoGitRemote) {
		t.Errorf("detectAppFromGit outside a repo = %v, want errNoGitRemote", err)
	}
}

func TestResolveAppSlugLocalOverGit(t *testing.T) {
	root := initTestGitRepo(t)
	if err := os.WriteFile(filepath.Join(root, config.LocalConfigFileName), []byte("app: local-app\n"), 0644); err != nil {
		t.Fatal(err)
	}
	runGitInDir(t, root, "remote", "add", "origin", "https://github.com/owner/repo.git")

	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("app", "", "")
	slug, err := resolveAppSlug(context.Background(), cmd, nil)
	if err != nil {
		t.Fatal(err)
	}
	if slug != "local-app" {
		t.Fatalf("slug = %q, want local-app", slug)
	}
}

func TestResolveAppSlugNoSource(t *testing.T) {
	orig, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(orig) })
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("app", "", "")
	_, err := resolveAppSlug(context.Background(), cmd, nil)
	if err == nil {
		t.Fatal("expected error when no app source available")
	}
}

func initTestGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitInDir(t, dir, "init")
	runGitInDir(t, dir, "config", "user.email", "test@example.com")
	runGitInDir(t, dir, "config", "user.name", "test")
	return dir
}

func runGitInDir(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v in %s: %v\n%s", args, dir, err, out)
	}
}

func TestIsBenignGitError(t *testing.T) {
	benign := []string{
		"fatal: not a git repository (or any of the parent directories): .git",
		"error: No such remote 'origin'",
		"fatal: No such file or directory",
	}
	for _, s := range benign {
		if !isBenignGitError(errors.New("exit status 1"), s) {
			t.Errorf("isBenignGitError(%q) = false, want true", s)
		}
	}
	if isBenignGitError(errors.New("exit status 128"), "fatal: could not read Username: Permission denied") {
		t.Error("permission error classified as benign; should surface")
	}
	if !isBenignGitError(exec.ErrNotFound, "") {
		t.Error("exec.ErrNotFound should be benign")
	}
}

func TestNormalizeGitURL(t *testing.T) {
	want := "github.com/owner/repo"
	cases := []string{
		"git@github.com:owner/repo.git",
		"git@github.com:owner/repo",
		"https://github.com/owner/repo.git",
		"https://github.com/owner/repo",
		"https://www.github.com/owner/repo.git",
		"https://GitHub.com/Owner/Repo.git",
		"  https://github.com/owner/repo.git  ",
	}
	for _, in := range cases {
		if got := normalizeGitURL(in); got != want {
			t.Errorf("normalizeGitURL(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseStatusFilter(t *testing.T) {
	nilCases := []string{""}
	for _, in := range nilCases {
		got, err := parseStatusFilter(in)
		if err != nil || got != nil {
			t.Errorf("parseStatusFilter(%q) = (%v, %v), want (nil, nil)", in, got, err)
		}
	}

	cases := []struct {
		name string
		want api.BuildStatus
	}{
		{"success", api.StatusSuccess},
		{"failed", api.StatusError},
		{"failure", api.StatusError},
		{"error", api.StatusError},
		{"running", api.StatusRunning},
		{"in-progress", api.StatusRunning},
		{"aborted", api.StatusAborted},
		{"SUCCESS", api.StatusSuccess},
	}
	for _, c := range cases {
		got, err := parseStatusFilter(c.name)
		if err != nil {
			t.Errorf("parseStatusFilter(%q): unexpected error %v", c.name, err)
			continue
		}
		if got == nil || *got != c.want {
			t.Errorf("parseStatusFilter(%q) = %v, want %v", c.name, got, c.want)
		}
	}

	if _, err := parseStatusFilter("typo"); err == nil {
		t.Error("parseStatusFilter(\"typo\"): expected error, got nil")
	}
}

func TestParseJSONFields(t *testing.T) {
	valid := validBuildFields()
	for _, in := range []string{"", "*", "all"} {
		got, err := parseJSONFields(in, valid)
		if err != nil {
			t.Errorf("parseJSONFields(%q): unexpected error %v", in, err)
		}
		if got != nil {
			t.Errorf("parseJSONFields(%q) = %v, want nil (all fields)", in, got)
		}
	}

	got, err := parseJSONFields("status, buildNumber ,", valid)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || !got["status"] || !got["buildNumber"] {
		t.Errorf("parseJSONFields subset = %v, want {status, buildNumber}", got)
	}

	if _, err := parseJSONFields("status,bogus", valid); err == nil {
		t.Error("parseJSONFields with unknown field: expected error, got nil")
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		secs int
		want string
	}{
		{5, "5s"},
		{59, "59s"},
		{60, "1m0s"},
		{125, "2m5s"},
		{3600, "1h0m"},
		{3725, "1h2m"},
	}
	for _, c := range cases {
		if got := formatDuration(c.secs); got != c.want {
			t.Errorf("formatDuration(%d) = %q, want %q", c.secs, got, c.want)
		}
	}
}

func TestTimeAgo(t *testing.T) {
	now := time.Now()
	cases := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "just now"},
		{5 * time.Minute, "5m ago"},
		{3 * time.Hour, "3h ago"},
		{50 * time.Hour, "2d ago"},
	}
	for _, c := range cases {
		if got := timeAgo(now.Add(-c.d)); got != c.want {
			t.Errorf("timeAgo(-%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestElapsed(t *testing.T) {
	now := time.Now()
	if got := elapsed(now.Add(-30 * time.Second)); got != "30s elapsed" {
		t.Errorf("elapsed(30s) = %q, want %q", got, "30s elapsed")
	}
	if got := elapsed(now.Add(-5 * time.Minute)); got != "5m elapsed" {
		t.Errorf("elapsed(5m) = %q, want %q", got, "5m elapsed")
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("truncate under limit = %q, want %q", got, "hello")
	}
	if got := truncate("line1\nline2", 20); got != "line1 line2" {
		t.Errorf("truncate newline = %q, want %q", got, "line1 line2")
	}
	got := truncate("abcdefghij", 5)
	if []rune(got)[len([]rune(got))-1] != '…' {
		t.Errorf("truncate over limit = %q, want trailing ellipsis", got)
	}
	if n := len([]rune(got)); n != 5 {
		t.Errorf("truncate over limit rune count = %d, want 5", n)
	}
	jp := truncate("あいうえおかきくけこ", 4)
	if n := len([]rune(jp)); n != 4 {
		t.Errorf("truncate multibyte rune count = %d, want 4", n)
	}
}
