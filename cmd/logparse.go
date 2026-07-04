package cmd

import (
	"regexp"
	"strconv"
	"strings"
)

// logStep is one step section of a Bitrise build log.
type logStep struct {
	Name     string
	ExitCode int
	Body     string // full section text, including the header line
}

var (
	stepHeaderRe = regexp.MustCompile(`^\s*\|\s*\(\d+\)\s+(.+?)(?:\s*\|)?\s*$`)
	exitCodeRe   = regexp.MustCompile(`(?i)exit.?code[:=\s]+(\d+)`)
)

// parseLogSteps splits a build log into per-step sections. It is the single
// source of truth for both `build view` (step summary) and
// `build logs --failed-only` (raw failed sections), so the two never disagree.
func parseLogSteps(logText string) []logStep {
	var steps []logStep
	var cur *logStep
	var body []string
	flush := func() {
		if cur != nil {
			cur.Body = strings.Join(body, "\n")
			steps = append(steps, *cur)
		}
	}
	for _, line := range strings.Split(logText, "\n") {
		if m := stepHeaderRe.FindStringSubmatch(line); m != nil {
			flush()
			cur = &logStep{Name: strings.TrimSpace(m[1])}
			body = []string{line}
			continue
		}
		if cur == nil {
			continue
		}
		body = append(body, line)
		if cur.ExitCode == 0 {
			if em := exitCodeRe.FindStringSubmatch(line); em != nil {
				cur.ExitCode, _ = strconv.Atoi(em[1])
			}
		}
	}
	flush()
	return steps
}

func failedSteps(steps []logStep) []logStep {
	var failed []logStep
	for _, s := range steps {
		if s.ExitCode != 0 {
			failed = append(failed, s)
		}
	}
	return failed
}

// failedStepLog returns the concatenated log sections of only the failed steps.
func failedStepLog(logText string) string {
	var sb strings.Builder
	for _, s := range failedSteps(parseLogSteps(logText)) {
		sb.WriteString(s.Body)
		if !strings.HasSuffix(s.Body, "\n") {
			sb.WriteByte('\n')
		}
	}
	return sb.String()
}
