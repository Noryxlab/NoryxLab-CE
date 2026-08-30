package handlers

import "testing"

// The overall status is what a badge in the interface renders, so a single
// critical alert must not be diluted by a list of warnings around it.
func TestOverallHealthTakesTheWorstAlert(t *testing.T) {
	cases := []struct {
		name   string
		alerts []healthAlert
		want   string
	}{
		{"no alerts", nil, "healthy"},
		{"empty list", []healthAlert{}, "healthy"},
		{"info only", []healthAlert{{Severity: healthInfo}}, "healthy"},
		{"one warning", []healthAlert{{Severity: healthWarning}}, "degraded"},
		{"one critical", []healthAlert{{Severity: healthCritical}}, "critical"},
		{
			"critical buried under warnings",
			[]healthAlert{{Severity: healthWarning}, {Severity: healthWarning}, {Severity: healthCritical}},
			"critical",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := overallHealth(tc.alerts); got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}
