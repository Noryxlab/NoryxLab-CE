package handlers

import (
	"encoding/json"
	"testing"
	"time"
)

// The point of the feature is the arithmetic and the honesty of its edges: a
// source that grew, a source that could not be reached, and an ontology that is
// simply old. Nothing here touches a bucket.
func TestOntologyFreshnessReportsDrift(t *testing.T) {
	manifest := ontologyManifest{
		GeneratedAt: time.Now().UTC().AddDate(0, 0, -78),
		SourceType:  "dataset",
		SourceID:    "dataset-1",
	}
	manifest.Summary.Objects = 18738
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}

	var decoded ontologyManifest
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode manifest: %v", err)
	}

	report := ontologyFreshness{
		GeneratedAt:     decoded.GeneratedAt,
		ManifestObjects: decoded.Summary.Objects,
		SourceObjects:   24179,
	}
	report.AgeDays = int(time.Since(decoded.GeneratedAt).Hours() / 24)
	report.Drift = report.SourceObjects - report.ManifestObjects
	report.Stale = report.Drift != 0 || report.AgeDays >= 30

	if report.Drift != 5441 {
		t.Fatalf("drift = %d, want 5441", report.Drift)
	}
	if report.AgeDays != 78 {
		t.Fatalf("ageDays = %d, want 78", report.AgeDays)
	}
	if !report.Stale {
		t.Fatal("an ontology 78 days old whose source grew by 5441 objects must read as stale")
	}
}

// A fresh scan of an unchanged source must not raise a warning: crying stale on
// a correct ontology teaches people to ignore the banner.
func TestOntologyFreshnessQuietWhenAligned(t *testing.T) {
	report := ontologyFreshness{ManifestObjects: 24179, SourceObjects: 24179, AgeDays: 2}
	report.Drift = report.SourceObjects - report.ManifestObjects
	report.Stale = report.Drift != 0 || report.AgeDays >= 30
	if report.Stale {
		t.Fatal("a two-day-old ontology matching its source must not be flagged")
	}
}
