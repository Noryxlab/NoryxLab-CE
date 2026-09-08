package handlers

import (
	"strings"
	"testing"
)

// A scan of a dataset laid out differently used to report "24,179 objects, 0
// subjects" and drop every path in silence. That reads as a broken feature
// rather than as "this layout is not the one the profile knows", which is what
// it is.
func TestAPathTheProfileCannotReadIsDescribedRatherThanDropped(t *testing.T) {
	// The convention the profile knows.
	shape := describePathShape("PREMYOM1000-0001/20260115/ANTERION/scan.dcm")
	if !strings.Contains(shape, "subject") || !strings.Contains(shape, "date") {
		t.Errorf("the known layout should be recognised: %s", shape)
	}

	// A deeper one, of the kind HDS-For holds: ten levels, the subject buried.
	deep := describePathShape("site-a/study/PREMYOM1000-0002/exam/20260115/device/series/1/2/image.dcm")
	if !strings.HasPrefix(deep, "10 levels") {
		t.Errorf("the depth should lead: %s", deep)
	}
	if !strings.Contains(deep, "DICOM") {
		t.Errorf("the format should be named: %s", deep)
	}

	// And nothing identifying leaves the scan: these are health-context
	// metadata, and a diagnosis does not need the identifiers.
	for _, identifier := range []string{"PREMYOM1000-0002", "site-a", "image.dcm", "20260115"} {
		if strings.Contains(deep, identifier) {
			t.Errorf("the shape leaks %q: %s", identifier, deep)
		}
	}
}

// Only the frequent shapes are worth showing, and a long tail must not push
// them off the list.
func TestOnlyTheFrequentShapesAreShown(t *testing.T) {
	layouts := map[string]int{}
	for index := 0; index < 20; index++ {
		layouts[strings.Repeat("x", index+1)] = index + 1
	}
	described := describeLayouts(layouts)
	if len(described) != 8 {
		t.Fatalf("expected the eight commonest, got %d", len(described))
	}
	if !strings.Contains(described[0], "(20)") {
		t.Errorf("the commonest should lead: %s", described[0])
	}
}

func TestTheCommonestUnreadableLayoutComesFirst(t *testing.T) {
	described := describeLayouts(map[string]int{
		"4 levels · text/text/text/file":                                 12,
		"10 levels · text/text/text/text/text/text/text/text/text/DICOM": 19022,
		"2 levels · text/CSV":                                            300,
	})
	if len(described) != 3 {
		t.Fatalf("expected three shapes, got %d", len(described))
	}
	if !strings.HasPrefix(described[0], "10 levels") || !strings.Contains(described[0], "(19022)") {
		t.Errorf("the commonest shape should lead, with its count: %s", described[0])
	}
	if describeLayouts(nil) != nil {
		t.Error("no unreadable path means nothing to describe")
	}
}
