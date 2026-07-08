package cmd

import (
	"testing"
	"time"

	"github.com/novr/bitrise-cli/internal/api"
)

func TestNeedsFailedStepLog(t *testing.T) {
	if !needsFailedStepLog(nil) {
		t.Error("nil requested should fetch failed steps")
	}
	if needsFailedStepLog(map[string]bool{"status": true}) {
		t.Error("status-only should not fetch failed steps")
	}
	if !needsFailedStepLog(map[string]bool{"failedSteps": true}) {
		t.Error("failedSteps requested should fetch log")
	}
}

func TestBuildViewToFieldMap(t *testing.T) {
	now := time.Now()
	build := api.Build{
		BuildNumber:       42,
		Branch:            "main",
		TriggeredWorkflow: "primary",
		Status:            api.StatusError,
		StatusText:        "error",
		TriggeredAt:       now,
	}
	failed := []logStep{{Name: "run-tests", ExitCode: 1}}
	m := buildViewToFieldMap(build, failed)

	if m["buildNumber"] != 42 {
		t.Fatalf("buildNumber = %v, want 42", m["buildNumber"])
	}
	steps, ok := m["failedSteps"].([]map[string]interface{})
	if !ok || len(steps) != 1 {
		t.Fatalf("failedSteps = %#v, want one step", m["failedSteps"])
	}
	if steps[0]["name"] != "run-tests" || steps[0]["exitCode"] != 1 {
		t.Fatalf("failed step = %#v", steps[0])
	}
}

func TestValidBuildViewFieldsIncludesFailedSteps(t *testing.T) {
	fields := validBuildViewFields()
	for _, f := range fields {
		if f == "failedSteps" {
			return
		}
	}
	t.Fatalf("validBuildViewFields = %v, missing failedSteps", fields)
}
