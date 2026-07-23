package cmd

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/novr/bitrise-cli/internal/api"
)

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

func TestWatchIntervalValidation(t *testing.T) {
	if minWatchInterval != 3*time.Second {
		t.Fatalf("minWatchInterval = %v, want 3s", minWatchInterval)
	}
}
