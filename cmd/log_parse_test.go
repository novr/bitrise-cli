package cmd

import (
	"strings"
	"testing"
)

const sampleLog = `
+------------------------------------------------------------------------------+
| (1) activate-ssh-key |
+------------------------------------------------------------------------------+
Running the step...
Step succeeded

+------------------------------------------------------------------------------+
| (2) run-xcode-tests@2.4.1 |
+------------------------------------------------------------------------------+
Building for testing...
error: test failed
Exit code: 1
`

func TestParseStepResults(t *testing.T) {
	steps := parseStepResults(sampleLog)
	if len(steps) != 2 {
		t.Fatalf("parseStepResults found %d steps, want 2: %+v", len(steps), steps)
	}
	if steps[0].Name != "activate-ssh-key" || steps[0].ExitCode != 0 {
		t.Errorf("step[0] = %+v, want {activate-ssh-key, 0}", steps[0])
	}
	if steps[1].Name != "run-xcode-tests@2.4.1" || steps[1].ExitCode != 1 {
		t.Errorf("step[1] = %+v, want {run-xcode-tests@2.4.1, 1}", steps[1])
	}
}

func TestFilterFailed(t *testing.T) {
	failed := filterFailed(parseStepResults(sampleLog))
	if len(failed) != 1 {
		t.Fatalf("filterFailed returned %d, want 1: %+v", len(failed), failed)
	}
	if failed[0].Name != "run-xcode-tests@2.4.1" || failed[0].ExitCode != 1 {
		t.Errorf("failed[0] = %+v, want {run-xcode-tests@2.4.1, 1}", failed[0])
	}
}

func TestExtractFailedStepSections(t *testing.T) {
	out := extractFailedStepSections(sampleLog)
	if out == "" {
		t.Fatal("expected failed section, got empty")
	}
	if !strings.Contains(out, "run-xcode-tests@2.4.1") {
		t.Errorf("failed section missing failing step name:\n%s", out)
	}
	if strings.Contains(out, "activate-ssh-key") {
		t.Errorf("failed section should not include the passing step:\n%s", out)
	}

	clean := `
+---+
| (1) ok-step |
+---+
Step succeeded
`
	if got := extractFailedStepSections(clean); got != "" {
		t.Errorf("clean log should yield empty, got:\n%s", got)
	}
}
