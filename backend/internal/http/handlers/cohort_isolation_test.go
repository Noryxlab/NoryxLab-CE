package handlers

import (
	"strings"
	"testing"
)

// Isolation is asked for, and an unknown answer never narrows anything.
//
// A mode nobody recognises must mean "the datasets, as before": silently
// giving somebody less than they could reach yesterday is the kind of change
// that looks like a bug for a week.
func TestOnlyCohortsIsolates(t *testing.T) {
	for _, mode := range []string{"cohorts", "Cohorts", " COHORTS "} {
		if !isolateToCohorts(mode) {
			t.Fatalf("%q must isolate", mode)
		}
	}
	for _, mode := range []string{"", "dataset", "datasets", "full", "isolated", "cohort"} {
		if isolateToCohorts(mode) {
			t.Fatalf("%q must not isolate", mode)
		}
	}
}

// The filler publishes a file only once it has landed.
//
// A tree whose paths error on open is worse than a tree that grows: the first
// teaches a person their data is broken, the second that it is arriving.
func TestTheFillerPublishesOnlyCompleteFiles(t *testing.T) {
	script := cohortFillerScript("/cache", "wks-1")
	// The object is written aside and renamed into the cache; the tree then
	// gets a hard link to it. A reader never opens a file still arriving, and
	// never sees a link to one.
	if !strings.Contains(script, `cp "$source" "$blob".partiel`) {
		t.Fatal("an object must be written aside before it is published")
	}
	if !strings.Contains(script, `mv "$blob".partiel "$blob"`) {
		t.Fatal("publishing must be a rename, which is what makes it atomic")
	}
	if !strings.Contains(script, `ln -f "$blob" "$cible"`) {
		t.Fatal("the tree must link to the published object, never to a partial one")
	}
	if strings.Contains(script, "/datasets/\"$dataset\"/\"$path\" \"$cible\"") {
		t.Fatal("no copy may target the dataset side")
	}
}

// A file already in the cache is linked, not fetched again.
//
// The objects are shared by the whole installation and the tree is per
// workspace: that is what makes a second team working on the same modality
// free, and what stops a workspace from seeing every cohort ever filled.
func TestAnObjectAlreadyCachedIsLinkedRatherThanFetched(t *testing.T) {
	script := cohortFillerScript("/cache", "wks-1")
	if !strings.Contains(script, "objets=") || !strings.Contains(script, "/trees/") {
		t.Fatal("objects and trees must live apart in the cache")
	}
	if !strings.Contains(script, `if [ -f "$blob" ]; then ln -f "$blob" "$cible"`) {
		t.Fatal("an object already cached must be linked instead of copied again")
	}
}

// And it fills in parallel, because parallelism is the whole difference:
// 14 MB/s in one stream against 81 with several, measured on 2026-09-26.
func TestTheFillerCopiesInParallel(t *testing.T) {
	script := cohortFillerScript("/cache", "wks-1")
	if !strings.Contains(script, "wait") || !strings.Contains(script, ") &") {
		t.Fatal("the filler must run several lanes and wait for them")
	}
	if cohortFillerLanes < 2 {
		t.Fatal("one lane is not parallelism")
	}
}

// Nothing it does may write to the source. The dataset is mounted read-only in
// that container and in no other, but a script that tried would fail at
// runtime rather than here, which is too late to be useful.
func TestTheFillerNeverWritesToTheDataset(t *testing.T) {
	script := cohortFillerScript("/cache", "wks-1")
	for _, forbidden := range []string{`> /datasets`, `rm -rf /datasets`, `mkdir -p /datasets`} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("the filler must not write to the dataset: %q", forbidden)
		}
	}
}
