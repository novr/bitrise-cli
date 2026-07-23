package cmd

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestBuildLogsJSONAndFailedOnlyMutuallyExclusive(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().Bool("failed-only", false, "")
	cmd.Flags().String("json", "", "")
	_ = cmd.Flags().Set("json", "steps")
	_ = cmd.Flags().Set("failed-only", "true")
	err := runBuildLogs(cmd, []string{"123"})
	if err == nil || !strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("runBuildLogs() = %v, want mutually exclusive error", err)
	}
}

func TestBuildLogsToFieldMap(t *testing.T) {
	steps := parseLogSteps(readTestLog(t, "standard.log"))
	m := buildLogsToFieldMap(steps)

	all, ok := m[fieldSteps].([]map[string]interface{})
	if !ok || len(all) != 2 {
		t.Fatalf("steps = %#v, want 2 steps", m[fieldSteps])
	}
	if all[0]["name"] != "activate-ssh-key" || all[0]["exitCode"] != 0 {
		t.Fatalf("first step = %#v", all[0])
	}
	if all[1]["name"] != "run-xcode-tests@2.4.1" || all[1]["exitCode"] != 1 {
		t.Fatalf("second step = %#v", all[1])
	}

	failed, ok := m[fieldFailedStepLogs].([]map[string]interface{})
	if !ok || len(failed) != 1 {
		t.Fatalf("failedStepLogs = %#v, want one failed step", m[fieldFailedStepLogs])
	}
	if failed[0]["name"] != "run-xcode-tests@2.4.1" {
		t.Fatalf("failed step name = %#v", failed[0])
	}
	body, ok := failed[0]["body"].(string)
	if !ok || !strings.Contains(body, "Exit code: 1") {
		t.Fatalf("failed step body = %#v", failed[0]["body"])
	}
}

func TestPrintBuildLogsJSONParseFailure(t *testing.T) {
	stderr := captureStderr(t, func() {
		if err := printBuildLogsJSON("plain text without step headers", map[string]bool{fieldSteps: true}); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(stderr, parseLogStepsFailureMessage) {
		t.Fatalf("stderr = %q, want parse failure message", stderr)
	}
}

func TestValidBuildLogsFields(t *testing.T) {
	fields := validBuildLogsFields()
	want := map[string]bool{fieldSteps: true, fieldFailedStepLogs: true}
	if len(fields) != len(want) {
		t.Fatalf("validBuildLogsFields = %v, want %v", fields, want)
	}
	for _, f := range fields {
		if !want[f] {
			t.Fatalf("unexpected field %q", f)
		}
	}
}

func readTestLog(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/logs/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = old })
	fn()
	_ = w.Close()
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()
	return buf.String()
}
