package handlers

import (
	"testing"
	"time"
)

// "a449755a" is what the platform calls a revision; "revision 3" is what a
// person calls it. The number counts forward from the first build, so it never
// changes once assigned - a workload pinned to revision 2 keeps meaning the
// same thing after the fourth rebuild.
func TestRevisionsAreNumberedOldestFirstAndShownNewestFirst(t *testing.T) {
	base := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	item := &environmentItem{Revisions: []environmentRevision{
		{BuildID: "third", CreatedAt: base.Add(2 * time.Hour)},
		{BuildID: "first", CreatedAt: base},
		{BuildID: "second", CreatedAt: base.Add(time.Hour)},
	}}

	numberEnvironmentRevisions(item)

	if len(item.Revisions) != 3 {
		t.Fatalf("expected three revisions, got %d", len(item.Revisions))
	}
	// Newest first on screen.
	if item.Revisions[0].BuildID != "third" || item.Revisions[2].BuildID != "first" {
		t.Fatalf("expected newest first, got %s then %s", item.Revisions[0].BuildID, item.Revisions[2].BuildID)
	}
	// Numbered by age, not by position.
	byBuild := map[string]int{}
	for _, revision := range item.Revisions {
		byBuild[revision.BuildID] = revision.Number
	}
	for build, expected := range map[string]int{"first": 1, "second": 2, "third": 3} {
		if byBuild[build] != expected {
			t.Errorf("%s should be revision %d, got %d", build, expected, byBuild[build])
		}
	}
}
