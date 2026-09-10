package handlers

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

// The five steps, checked against what Kubernetes actually said.
//
// The hand-written cases above this file encode what we believed Kubernetes
// emits. These fixtures are what it emitted, captured from a live cluster by
// scripts/ops/capture-incidents.py, each from a failure caused on purpose so
// its cause is known rather than inferred. The distinction is not academic:
// the storage case below was diagnosed as a full cluster for as long as the
// fixtures did not exist.
//
// Refresh them with:
//
//	scripts/ops/capture-incidents.py --context noryx-test
type incidentFixture struct {
	ID     string `json:"id"`
	Label  string `json:"label"`
	Expect struct {
		Step   string `json:"step"`
		Detail string `json:"detail"`
		Stuck  bool   `json:"stuck"`
	} `json:"expect"`
	Recorded string                  `json:"recorded"`
	Status   noryxruntime.PodStatus  `json:"status"`
	Events   []noryxruntime.PodEvent `json:"events"`
}

func TestStartupReportDiagnosesRealIncidents(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("testdata", "incidents", "*.json"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("no incident fixtures found: capture them with scripts/ops/capture-incidents.py (%v)", err)
	}

	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			raw, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading %s: %v", path, err)
			}
			var fixture incidentFixture
			if err := json.Unmarshal(raw, &fixture); err != nil {
				t.Fatalf("parsing %s: %v", path, err)
			}

			report := buildStartupReport(fixture.Recorded, fixture.Status, fixture.Events)

			if report.Stuck != fixture.Expect.Stuck {
				t.Errorf("%s: stuck = %v, want %v - the interface uses this to stop telling a reader that waiting will help",
					fixture.Label, report.Stuck, fixture.Expect.Stuck)
			}

			var got string
			for _, step := range report.Steps {
				if step.Detail != "" {
					got = step.Key + "/" + step.Detail
				}
			}
			want := ""
			if fixture.Expect.Detail != "" {
				want = fixture.Expect.Step + "/" + fixture.Expect.Detail
			}
			if got != want {
				t.Errorf("%s: diagnosed %q, want %q\nA wrong diagnosis is worse than none: it sends the reader, and any agent reading this report, after the wrong cause.",
					fixture.Label, got, want)
			}
		})
	}
}
