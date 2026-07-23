package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/novr/bitrise-cli/internal/api"
	"github.com/spf13/cobra"
)

func executeBuildWatch(t *testing.T, args ...string) error {
	t.Helper()
	full := append([]string{"build", "watch"}, args...)
	rootCmd.SetArgs(full)
	rootCmd.SetContext(context.Background())
	return rootCmd.Execute()
}

func TestWatchBuildAlreadyFinished(t *testing.T) {
	build := &api.Build{
		BuildNumber:       42,
		Branch:            "main",
		TriggeredWorkflow: "primary",
		Status:            api.StatusSuccess,
		StatusText:        "success",
	}
	calls := 0
	orig := getBuildForWatch
	getBuildForWatch = func(ctx context.Context, client *api.Client, appSlug string, buildNumber int) (*api.Build, error) {
		calls++
		return build, nil
	}
	t.Cleanup(func() { getBuildForWatch = orig })

	var buf bytes.Buffer
	got, err := watchBuild(context.Background(), nil, "app", 42, 3*time.Second, &buf, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != api.StatusSuccess {
		t.Fatalf("status = %v, want success", got.Status)
	}
	if calls != 1 {
		t.Fatalf("GetBuild calls = %d, want 1", calls)
	}
	if !strings.Contains(buf.String(), "#42") {
		t.Fatalf("output = %q, want build number", buf.String())
	}
}

func TestWatchBuildPollsUntilFinished(t *testing.T) {
	running := &api.Build{
		BuildNumber:       7,
		Branch:            "main",
		TriggeredWorkflow: "primary",
		Status:            api.StatusRunning,
		StatusText:        "running",
		TriggeredAt:       time.Now().Add(-30 * time.Second),
	}
	success := *running
	success.Status = api.StatusSuccess
	success.StatusText = "success"

	calls := 0
	orig := getBuildForWatch
	getBuildForWatch = func(ctx context.Context, client *api.Client, appSlug string, buildNumber int) (*api.Build, error) {
		calls++
		if calls == 1 {
			return running, nil
		}
		return &success, nil
	}
	t.Cleanup(func() { getBuildForWatch = orig })

	var buf bytes.Buffer
	got, err := watchBuild(context.Background(), nil, "app", 7, 10*time.Millisecond, &buf, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != api.StatusSuccess {
		t.Fatalf("status = %v, want success", got.Status)
	}
	if calls != 2 {
		t.Fatalf("GetBuild calls = %d, want 2", calls)
	}
}

func TestBuildWatchExitError(t *testing.T) {
	build := api.Build{BuildNumber: 9, Status: api.StatusSuccess, StatusText: "success"}
	if err := buildWatchExitError(true, build); err != nil {
		t.Fatalf("success build: %v", err)
	}
	if err := buildWatchExitError(false, build); err != nil {
		t.Fatalf("success build no flag: %v", err)
	}

	failed := api.Build{BuildNumber: 9, Status: api.StatusError, StatusText: "error"}
	if err := buildWatchExitError(false, failed); err != nil {
		t.Fatalf("failed build no flag: %v", err)
	}
	if err := buildWatchExitError(true, failed); err == nil {
		t.Fatal("expected error with --exit-status on failed build")
	}

	aborted := api.Build{BuildNumber: 9, Status: api.StatusAborted, StatusText: "aborted"}
	if err := buildWatchExitError(true, aborted); err == nil {
		t.Fatal("expected error with --exit-status on aborted build")
	}
}

func TestValidateWatchInterval(t *testing.T) {
	if err := validateWatchInterval(2 * time.Second); err == nil {
		t.Fatal("expected error for short interval")
	}
	if err := validateWatchInterval(3 * time.Second); err != nil {
		t.Fatalf("3s interval: %v", err)
	}
}

func TestRunBuildWatchRejectsShortInterval(t *testing.T) {
	err := executeBuildWatch(t, "1", "--interval", "1s")
	if err == nil || !strings.Contains(err.Error(), "invalid --interval") {
		t.Fatalf("err = %v, want invalid --interval", err)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = old })
	fn()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String()
}

func TestRunBuildWatchJSONSkipsPollOutput(t *testing.T) {
	build := &api.Build{
		BuildNumber:       5,
		Branch:            "main",
		TriggeredWorkflow: "primary",
		Status:            api.StatusSuccess,
		StatusText:        "success",
	}
	orig := getBuildForWatch
	getBuildForWatch = func(ctx context.Context, client *api.Client, appSlug string, buildNumber int) (*api.Build, error) {
		return build, nil
	}
	t.Cleanup(func() { getBuildForWatch = orig })

	origResolve := resolveAppSlugForWatch
	resolveAppSlugForWatch = func(ctx context.Context, cmd *cobra.Command, client *api.Client) (string, error) {
		return "test-app", nil
	}
	t.Cleanup(func() { resolveAppSlugForWatch = origResolve })

	origClient := newAPIClientForWatch
	newAPIClientForWatch = func() (*api.Client, error) {
		return api.NewClient("test-token"), nil
	}
	t.Cleanup(func() { newAPIClientForWatch = origClient })

	var stderr bytes.Buffer
	rootCmd.SetErr(&stderr)
	out := captureStdout(t, func() {
		if err := executeBuildWatch(t, "5", "--app", "test-app", "--interval", "5s", "--json", "status,buildNumber"); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(stderr.String(), "⟳") || strings.Contains(stderr.String(), "✓") {
		t.Fatalf("poll status leaked into stderr: %q", stderr.String())
	}
	var row map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &row); err != nil {
		t.Fatalf("invalid json: %v\n%s", err, out)
	}
	if row["buildNumber"] != float64(5) {
		t.Fatalf("buildNumber = %v, want 5", row["buildNumber"])
	}
}

func TestIsWriterTerminal(t *testing.T) {
	if isWriterTerminal(&bytes.Buffer{}) {
		t.Error("bytes.Buffer should not be a terminal")
	}
}
