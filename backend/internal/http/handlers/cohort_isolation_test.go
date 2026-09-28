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
	script := cohortFillerScript("/cache")
	if !strings.Contains(script, `cp "$source" "$cible".partiel`) {
		t.Fatal("a file must be written aside before it is published")
	}
	if !strings.Contains(script, `mv "$cible".partiel "$cible"`) {
		t.Fatal("publishing must be a rename, which is what makes it atomic")
	}
	if strings.Contains(script, "/datasets/\"$dataset\"/\"$path\" \"$cible\"") {
		t.Fatal("no copy may target the dataset side")
	}
}

// And it fills in parallel, because parallelism is the whole difference:
// 14 MB/s in one stream against 81 with several, measured on 2026-09-26.
func TestTheFillerCopiesInParallel(t *testing.T) {
	script := cohortFillerScript("/cache")
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
	script := cohortFillerScript("/cache")
	for _, forbidden := range []string{`> /datasets`, `rm -rf /datasets`, `mkdir -p /datasets`} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("the filler must not write to the dataset: %q", forbidden)
		}
	}
}
