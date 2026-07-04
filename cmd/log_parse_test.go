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

func TestParseLogSteps(t *testing.T) {
	steps := parseLogSteps(sampleLog)
	if len(steps) != 2 {
		t.Fatalf("parseLogSteps found %d steps, want 2: %+v", len(steps), steps)
	}
	if steps[0].Name != "activate-ssh-key" || steps[0].ExitCode != 0 {
		t.Errorf("step[0] = {%s, %d}, want {activate-ssh-key, 0}", steps[0].Name, steps[0].ExitCode)
	}
	if steps[1].Name != "run-xcode-tests@2.4.1" || steps[1].ExitCode != 1 {
		t.Errorf("step[1] = {%s, %d}, want {run-xcode-tests@2.4.1, 1}", steps[1].Name, steps[1].ExitCode)
	}
}

func TestFailedSteps(t *testing.T) {
	failed := failedSteps(parseLogSteps(sampleLog))
	if len(failed) != 1 {
		t.Fatalf("failedSteps returned %d, want 1", len(failed))
	}
	if failed[0].Name != "run-xcode-tests@2.4.1" {
		t.Errorf("failed[0].Name = %s, want run-xcode-tests@2.4.1", failed[0].Name)
	}
}

func TestJoinStepBodies(t *testing.T) {
	out := joinStepBodies(failedSteps(parseLogSteps(sampleLog)))
	if !strings.Contains(out, "run-xcode-tests@2.4.1") {
		t.Errorf("output missing failing step name:\n%s", out)
	}
	if strings.Contains(out, "activate-ssh-key") {
		t.Errorf("output should not include the passing step:\n%s", out)
	}
}

// Multi-step logs with interspersed non-header pipe lines, a cache-hit info
// step, and a failure signalled several lines after the header.
func TestParseLogStepsMultiStep(t *testing.T) {
	const log = `
Preamble output before any step (should not become a step)
| this pipe line is not a step header |

+---+
| (1) git-clone@8 |
+---+
Cloning...
| some table row inside step output |
exit code: 0

+---+
| (2) cache-pull@2 |
+---+
Cache hit, restored 120MB

+---+
| (3) fastlane@3.1.0 |
+---+
Running lane: test
** BUILD FAILED **
Exit code: 65
`
	steps := parseLogSteps(log)
	if len(steps) != 3 {
		t.Fatalf("got %d steps, want 3: %+v", len(steps), stepNames(steps))
	}
	names := stepNames(steps)
	want := []string{"git-clone@8", "cache-pull@2", "fastlane@3.1.0"}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("step[%d] name = %s, want %s", i, names[i], want[i])
		}
	}
	failed := failedSteps(steps)
	if len(failed) != 1 || failed[0].Name != "fastlane@3.1.0" || failed[0].ExitCode != 65 {
		t.Errorf("failed = %+v, want only fastlane@3.1.0 exit 65", failed)
	}
	// The passing step's "exit code: 0" must not mark it failed.
	if steps[0].ExitCode != 0 {
		t.Errorf("git-clone ExitCode = %d, want 0", steps[0].ExitCode)
	}
}

// A fully successful build must yield zero failed steps (parser still finds steps).
func TestParseLogStepsAllPass(t *testing.T) {
	const log = `
+---+
| (1) script@1 |
+---+
hello
exit code: 0
`
	steps := parseLogSteps(log)
	if len(steps) != 1 {
		t.Fatalf("got %d steps, want 1", len(steps))
	}
	if n := len(failedSteps(steps)); n != 0 {
		t.Errorf("failedSteps = %d, want 0", n)
	}
}

// A log the parser cannot segment yields no steps (drives the --failed-only
// "could not identify steps" branch).
func TestParseLogStepsUnrecognized(t *testing.T) {
	if steps := parseLogSteps("just some unstructured output\nwith no step headers\n"); len(steps) != 0 {
		t.Errorf("got %d steps, want 0: %+v", len(steps), stepNames(steps))
	}
}

func stepNames(steps []logStep) []string {
	names := make([]string, len(steps))
	for i, s := range steps {
		names[i] = s.Name
	}
	return names
}
