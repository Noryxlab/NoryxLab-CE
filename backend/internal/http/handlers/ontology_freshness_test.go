package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
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

// The freshness count must count what the scan counts.
//
// It counted every key, including the zero-byte directory markers the scan
// deliberately skips, so the two figures could never agree. SELENA was listed
// at 4,111 against a manifest of 3,994 and declared "no longer describes its
// source" the instant it was created, by exactly its 117 directory keys.
// PREMYOM1000 was permanently stale by 3. A warning that is always on is a
// warning nobody reads on the day the study really does recruit.
func TestFreshnessCountsWhatTheScanCounts(t *testing.T) {
	keys := []struct {
		key  string
		size int64
	}{
		{"SELENA/", 0},
		{"SELENA/SELENA-01-001/", 0},
		{"SELENA/SELENA-01-001/20260218/", 0},
		{"SELENA/SELENA-01-001/20260218/ANTERION/scan.dcm", 4096},
		{"SELENA/SELENA-01-001/20260218/IRM/scan.dcm", 8192},
		{"SELENA/checksums/", 0},
		{"SELENA/checksums/checksum_20260918.tsv", 128},
	}
	listing := make(chan minio.ObjectInfo, len(keys))
	for _, entry := range keys {
		listing <- minio.ObjectInfo{Key: entry.key, Size: entry.size}
	}
	close(listing)

	count, err := countListedObjects(listing)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	// Three files. The four directory markers are not objects, and counting
	// them is what made every ontology stale from birth.
	if count != 3 {
		t.Fatalf("counted %d objects, want 3", count)
	}
}

// A zero-byte file is a file. The lookahead must not swallow one just because
// it is empty: only a key that prefixes the next one is a directory marker.
func TestAnEmptyFileIsNotADirectoryKey(t *testing.T) {
	listing := make(chan minio.ObjectInfo, 2)
	listing <- minio.ObjectInfo{Key: "study/subject/empty.txt", Size: 0}
	listing <- minio.ObjectInfo{Key: "study/subject/scan.dcm", Size: 4096}
	close(listing)

	count, err := countListedObjects(listing)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("counted %d objects, want 2", count)
	}
}
