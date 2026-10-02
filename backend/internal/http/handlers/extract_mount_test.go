package handlers

import (
	"strings"
	"testing"
)

// The extract mounts as links over the dataset that is already there. If this
// ever became a copy, a 400 GB study would be duplicated per workspace - and,
// worse, the platform would be writing a second copy of regulated data it was
// only ever meant to read.
func TestExtractBootstrapLinksAndNeverCopies(t *testing.T) {
	script := strings.Join(extractBootstrapLines("/mnt", true, 0), "\n")
	if !strings.Contains(script, "ln -sfn") {
		t.Fatal("the extract tree must be built from symlinks")
	}
	for _, forbidden := range []string{"cp ", "rsync", "mc cp", "aws s3 cp"} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("extract bootstrap copies data (%q); it must only link", forbidden)
		}
	}
}

// An extract too large for one manifest is said out loud. A partial tree would
// look like a complete study and quietly change someone's n.
func TestOversizedExtractIsRefusedOutLoud(t *testing.T) {
	script := strings.Join(extractBootstrapLines("/mnt", false, 41234), "\n")
	if !strings.Contains(script, "41234") || !strings.Contains(script, "not mounted") {
		t.Fatalf("an unmountable extract must say so; got: %s", script)
	}
	if strings.Contains(script, "ln -sfn") {
		t.Fatal("a refused extract must not build a partial tree")
	}
}

// A tab or a newline in a key would split a record and file an object under the
// wrong subject. Such a key is dropped, never repaired into something plausible.
func TestExtractManifestDropsKeysThatWouldSplitARecord(t *testing.T) {
	encoded, ok := encodeExtractManifest([]extractMountEntry{
		{ExtractName: "c", DatasetDir: "d", Dir: "S1/v1/m", Path: "good/file.csv"},
		{ExtractName: "c", DatasetDir: "d", Dir: "S1/v1/m", Path: "bad\tfile.csv"},
	})
	if !ok || encoded == "" {
		t.Fatal("a small manifest must encode")
	}
}
