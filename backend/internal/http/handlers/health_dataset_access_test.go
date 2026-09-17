package handlers

import (
	"strings"
	"testing"
)

// The summary an operator reads first.
//
// Written against the wording rather than the plumbing: probing an object
// store needs one, and what actually failed here was not the probe but the
// silence around it - a dataset that stopped being readable, and a home page
// that blamed the health-data exclusion for the gap.
func TestTheSummaryCountsWhatItNames(t *testing.T) {
	for _, tc := range []struct {
		name        string
		unreachable []string
		wantSummary string
		wantNamed   int
	}{
		{"one", []string{"imt"}, "1 dataset cannot be read", 1},
		{"a few", []string{"imt", "essilor-wp2", "for"}, "3 datasets cannot be read", 3},
		// Names that cannot occur in the surrounding prose: single letters
		// matched the message itself and failed this test for the wrong
		// reason, which is a fixture bug wearing a defect's clothes.
		{"too many to list", []string{"zeta-1", "zeta-2", "zeta-3", "zeta-4", "zeta-5"}, "5 datasets cannot be read", 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			alert := datasetAccessAlert(tc.unreachable)
			if alert.Summary != tc.wantSummary {
				t.Errorf("summary %q, want %q", alert.Summary, tc.wantSummary)
			}
			named := 0
			for _, name := range tc.unreachable {
				if strings.Contains(alert.Detail, name) {
					named++
				}
			}
			if named > datasetNamesShown {
				t.Errorf("named %d datasets, the list should stop at %d", named, datasetNamesShown)
			}
			// Past the cap the count has to survive, or the operator reads
			// three names and believes that is all of them.
			if len(tc.unreachable) > datasetNamesShown && !strings.Contains(alert.Detail, "more") {
				t.Errorf("the truncated list does not say how many were left out: %q", alert.Detail)
			}
		})
	}
}

func TestTheDetailNamesTheLikelyCause(t *testing.T) {
	// The alert exists because the cause is guessable and the symptom is not:
	// a rotated credential looks exactly like an empty dataset from the
	// outside. Saying so is most of the value.
	alert := datasetAccessAlert([]string{"imt"})
	if !strings.Contains(alert.Detail, "credential") {
		t.Errorf("the detail does not mention credentials: %q", alert.Detail)
	}
	if !strings.Contains(alert.Detail, "silently exclude") {
		t.Errorf("the detail does not say what the platform gets wrong meanwhile: %q", alert.Detail)
	}
	if alert.Severity != healthWarning {
		t.Errorf("severity %q: nobody's work has stopped, and critical would train an operator to ignore it", alert.Severity)
	}
}
