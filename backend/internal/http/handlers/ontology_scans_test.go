package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
)

func manifestForScan(subjects, objects int, layout *dataset.PathLayout) []byte {
	manifest := ontologyManifest{Study: "SELENA", ReadingRule: describeReading(layout)}
	manifest.Summary.Subjects = subjects
	manifest.Summary.Objects = objects
	raw, _ := json.Marshal(manifest)
	return raw
}

// A history is read newest first and compared oldest first, and the oldest
// row has nothing to compare against.
//
// Inventing a baseline of zero there would report a first scan as a study
// that gained thirty-one subjects overnight, which is exactly the false alarm
// the feature exists to remove.
func TestTheScanHistoryComparesEachScanWithTheOneBeforeIt(t *testing.T) {
	base := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	scans := []ontologydomain.Scan{
		// Deliberately out of order: the comparison must not depend on the
		// order the store happened to return.
		ontologydomain.NewScan("o1", manifestForScan(31, 24179, nil), "stef", base.AddDate(0, 0, 2)),
		ontologydomain.NewScan("o1", manifestForScan(2, 3994, nil), "stef", base),
		ontologydomain.NewScan("o1", manifestForScan(20, 18738, nil), "stef", base.AddDate(0, 0, 1)),
	}

	entries := ontologyScanEntries(scans)
	if len(entries) != 3 {
		t.Fatalf("got %d entries, want 3", len(entries))
	}

	// Newest first for the reader.
	if entries[0].Subjects != 31 || entries[2].Subjects != 2 {
		t.Fatalf("history is not newest first: %d then %d", entries[0].Subjects, entries[2].Subjects)
	}
	if entries[0].Difference == nil {
		t.Fatal("the newest scan must say what it changed")
	}
	if entries[0].Difference.PreviousSubjects != 20 || entries[0].Difference.CurrentSubjects != 31 {
		t.Errorf("compared against the wrong scan: %d -> %d",
			entries[0].Difference.PreviousSubjects, entries[0].Difference.CurrentSubjects)
	}
	if entries[2].Difference != nil {
		t.Error("the oldest scan has nothing to compare against and must say nothing")
	}
}

// Two scans differ either because the data moved or because the reading
// changed, and those call for opposite reactions. A history that cannot tell
// them apart answers nothing.
func TestTheScanHistoryRecordsTheRuleThatProducedEachScan(t *testing.T) {
	declared := &dataset.PathLayout{
		SubjectLevel: 1, VisitLevel: 2, ModalityLevel: 3,
		SubjectName: "patient", VisitName: "visite", ModalityName: "modalité",
	}
	base := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	scans := []ontologydomain.Scan{
		ontologydomain.NewScan("o1", manifestForScan(2, 3994, nil), "stef", base),
		ontologydomain.NewScan("o1", manifestForScan(2, 3994, declared), "stef", base.Add(time.Hour)),
	}

	entries := ontologyScanEntries(scans)
	if entries[0].Reading.Source != "declared" || entries[1].Reading.Source != "default" {
		t.Fatalf("the rule did not travel with the scan: %q then %q",
			entries[0].Reading.Source, entries[1].Reading.Source)
	}
	if entries[0].Reading.SubjectName != "patient" {
		t.Errorf("a declared name must be kept: %q", entries[0].Reading.SubjectName)
	}
	if entries[0].Difference == nil || !entries[0].Difference.ReadingChanged {
		t.Error("a scan read differently must say so, or the counts look like news about the data")
	}
}

// The objects figure is what the reading produced, matching the card: a
// history that counts differently from the page above it is a history nobody
// can use.
func TestTheScanHistoryCountsWhatTheReadingProduced(t *testing.T) {
	manifest := ontologyManifest{Study: "SELENA"}
	manifest.Summary.Objects = 3994
	manifest.Summary.Unrecognised = 1
	raw, _ := json.Marshal(manifest)

	entries := ontologyScanEntries([]ontologydomain.Scan{
		ontologydomain.NewScan("o1", raw, "stef", time.Now().UTC()),
	})
	if entries[0].Objects != 3993 {
		t.Errorf("objects = %d, want 3993", entries[0].Objects)
	}
	if entries[0].Unreadable != 1 {
		t.Errorf("unreadable = %d, want 1", entries[0].Unreadable)
	}
}
