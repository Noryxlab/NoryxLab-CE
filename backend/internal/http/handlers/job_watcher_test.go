package handlers

import (
	"strings"
	"testing"
)

// Only a crossing into failure alerts. A status seen for the first time must
// not, or every restart would replay the entire history of past failures.
func TestJobFailureAlertsOnTransitionOnly(t *testing.T) {
	cases := []struct {
		name     string
		previous string
		current  string
		alerts   bool
	}{
		{"running then failed", "running", "failed", true},
		{"submitted then failed", "submitted", "failed", true},
		{"already failed", "failed", "failed", false},
		{"succeeded stays succeeded", "succeeded", "succeeded", false},
		{"failed then rerun succeeds", "failed", "succeeded", false},
		{"case is ignored", "Running", "FAILED", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			shouldAlert := isFailedJobStatus(tc.current) && !isFailedJobStatus(tc.previous)
			if shouldAlert != tc.alerts {
				t.Fatalf("%s -> %s: expected alert=%v, got %v", tc.previous, tc.current, tc.alerts, shouldAlert)
			}
		})
	}
}

// An alert is read on a phone: the tail of the log is where a stack trace
// ends, and it must stay short.
func TestLastLinesKeepsTheTail(t *testing.T) {
	logs := "ligne 1\n\nligne 2\nligne 3\n   \nligne 4\nligne 5\nligne 6\n"
	got := lastLines(logs, 3)
	if !strings.Contains(got, "ligne 6") || !strings.Contains(got, "ligne 4") {
		t.Fatalf("expected the final lines, got %q", got)
	}
	if strings.Contains(got, "ligne 1") {
		t.Fatalf("expected earlier lines to be dropped, got %q", got)
	}
	if strings.Count(got, "|") != 2 {
		t.Fatalf("expected exactly three lines joined, got %q", got)
	}
}
