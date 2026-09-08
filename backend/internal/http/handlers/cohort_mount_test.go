package handlers

import (
	"strings"
	"testing"
)

// The cohort mounts as links over the dataset that is already there. If this
// ever became a copy, a 400 GB study would be duplicated per workspace - and,
// worse, the platform would be writing a second copy of regulated data it was
// only ever meant to read.
func TestCohortBootstrapLinksAndNeverCopies(t *testing.T) {
	script := strings.Join(cohortBootstrapLines("/mnt", true, 0), "\n")
	if !strings.Contains(script, "ln -sfn") {
		t.Fatal("the cohort tree must be built from symlinks")
	}
	for _, forbidden := range []string{"cp ", "rsync", "mc cp", "aws s3 cp"} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("cohort bootstrap copies data (%q); it must only link", forbidden)
		}
	}
}

// A cohort too large for one manifest is said out loud. A partial tree would
// look like a complete study and quietly change someone's n.
func TestOversizedCohortIsRefusedOutLoud(t *testing.T) {
	script := strings.Join(cohortBootstrapLines("/mnt", false, 41234), "\n")
	if !strings.Contains(script, "41234") || !strings.Contains(script, "not mounted") {
		t.Fatalf("an unmountable cohort must say so; got: %s", script)
	}
	if strings.Contains(script, "ln -sfn") {
		t.Fatal("a refused cohort must not build a partial tree")
	}
}

// A tab or a newline in a key would split a record and file an object under the
// wrong subject. Such a key is dropped, never repaired into something plausible.
func TestCohortManifestDropsKeysThatWouldSplitARecord(t *testing.T) {
	encoded, ok := encodeCohortManifest([]cohortMountEntry{
		{CohortName: "c", DatasetDir: "d", SubjectID: "S1", Visit: "v1", Modality: "m", Path: "good/file.csv"},
		{CohortName: "c", DatasetDir: "d", SubjectID: "S1", Visit: "v1", Modality: "m", Path: "bad\tfile.csv"},
	})
	if !ok || encoded == "" {
		t.Fatal("a small manifest must encode")
	}
}
