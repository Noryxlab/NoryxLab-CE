package handlers

import (
	"strings"
	"testing"
)

// Isolation is asked for, and an unknown answer never narrows anything.
//
// The filler publishes a file only once it has landed.
//
// A tree whose paths error on open is worse than a tree that grows: the first
// teaches a person their data is broken, the second that it is arriving.
func TestTheFillerPublishesOnlyCompleteFiles(t *testing.T) {
	script := extractFillerScript("/cache", "wks-1")
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
// free, and what stops a workspace from seeing every extract ever filled.
func TestAnObjectAlreadyCachedIsLinkedRatherThanFetched(t *testing.T) {
	script := extractFillerScript("/cache", "wks-1")
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
	script := extractFillerScript("/cache", "wks-1")
	if !strings.Contains(script, "wait") || !strings.Contains(script, ") &") {
		t.Fatal("the filler must run several lanes and wait for them")
	}
	if extractFillerLanes < 2 {
		t.Fatal("one lane is not parallelism")
	}
}

// Nothing it does may write to the source. The dataset is mounted read-only in
// that container and in no other, but a script that tried would fail at
// runtime rather than here, which is too late to be useful.
func TestTheFillerNeverWritesToTheDataset(t *testing.T) {
	script := extractFillerScript("/cache", "wks-1")
	for _, forbidden := range []string{`> /datasets`, `rm -rf /datasets`, `mkdir -p /datasets`} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("the filler must not write to the dataset: %q", forbidden)
		}
	}
}

// A tree outlives nothing.
//
// A deleted workspace leaves its tree of hard links behind - nothing in the
// container can ask Kubernetes what still exists - and those links would keep
// objects alive that nobody wants, so eviction would free nothing. Age alone
// cannot say which tree is dead: a workspace open all day never touches its
// own. So the living filler renews a lease while it sleeps, and the next one
// to start reaps what has not been renewed.
func TestAnAbandonedTreeIsReaped(t *testing.T) {
	script := extractFillerScript("/cache", "wks-1")
	if !strings.Contains(script, `touch "$bail"`) {
		t.Fatal("a living filler must renew the lease on its own tree")
	}
	// Beside the tree, never inside it: a lease in the tree is a file in
	// somebody's extract directory, and 64 files where the extract holds 63.
	if strings.Contains(script, `"$root"/.vivant`) {
		t.Fatal("the lease must not live inside the tree somebody browses")
	}
	if !strings.Contains(script, `-mmin -120`) {
		t.Fatal("the reaper must decide on the lease, with grace")
	}
	if !strings.Contains(script, `[ "$arbre" = "$root" ] && continue`) {
		t.Fatal("a filler must never reap the tree it is filling")
	}
}

// The cache is pruned where it grows.
//
// It only fills when a workspace launches, so that is when pruning is worth
// doing - and it needs no second component, no image to keep in step and no
// schedule to forget. What may go is what no tree references: a link count of
// one means the copy in objects/ and nothing else.
func TestTheCacheIsPrunedAtLaunch(t *testing.T) {
	script := extractFillerScript("/cache", "wks-1")
	if !strings.Contains(script, "-links 1") {
		t.Fatal("only objects nothing references may be evicted")
	}
	if !strings.Contains(script, "sort -n") {
		t.Fatal("eviction must start with the oldest")
	}
	if extractCacheLowMark >= extractCacheHighMark {
		t.Fatal("eviction must stop below where it starts, or every launch evicts again")
	}
}

// A file keeps its place under the modality, not just its name.
//
// The tree placed each file under its basename, and DICOM names its slices by
// number: 00000121 exists in every series. On PREMYOM1000's ANTERION modality
// that turned 21 730 objects into 18 825 files - a study silently 13% smaller,
// with nothing logged and nothing refused. Measured on EMSE on 2026-09-28,
// after the first full-scale fill; it predates the side-car and would have
// shipped with the links.
func TestAFileKeepsItsPlaceBelowTheModality(t *testing.T) {
	cases := map[string]string{
		"PREMYOM1000/S-1/20250430/ANTERION/DICOM/MC22/343876/00000121":        "DICOM/MC22/343876/00000121",
		"old/S-1/visit_20250430/modality_ANTERION/DICOM/MC22/343879/00000121": "DICOM/MC22/343879/00000121",
		"PREMYOM1000/S-1/20250430/ANTERION/Cornea_Basics.csv":                 "Cornea_Basics.csv",
	}
	for path, want := range cases {
		if got := extractLeaf(path, "ANTERION"); got != want {
			t.Fatalf("%s: expected %q, got %q", path, want, got)
		}
	}
	// Two slices of different series no longer collide.
	a := extractLeaf("PREMYOM1000/S-1/20250430/ANTERION/DICOM/MC22/343876/00000121", "ANTERION")
	b := extractLeaf("PREMYOM1000/S-1/20250430/ANTERION/DICOM/MC22/343879/00000121", "ANTERION")
	if a == b {
		t.Fatal("two slices of different series must not land on the same path")
	}
	// A layout this does not recognise keeps its whole path: longer than it
	// needs to be, and never wrong.
	if got := extractLeaf("some/other/layout/file.dcm", "ANTERION"); got != "some/other/layout/file.dcm" {
		t.Fatalf("an unrecognised layout must keep its path, got %q", got)
	}
}
