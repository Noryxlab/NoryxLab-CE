package handlers

import "testing"

// A run records what it read, as it was at that moment.
//
// A project's attachments change and a result does not. Asked in two years
// which data produced a figure, a platform that can only answer "whatever is
// attached to that project today" has not answered - and for work that ends in
// a medical device, that question comes from somebody who does not take an
// approximation.
func TestAJobRecordsTheDatasetsItRead(t *testing.T) {
	mounted := []workspaceAttachedDataset{
		{ID: "d1", Name: "hds-essilor", Bucket: "hds-essilor.wp2", Prefix: "mri_test", ReadOnly: true},
		{ID: "d2", Name: "figures", Bucket: "noryx-ds-abc", ReadOnly: false},
	}

	recorded := jobDatasetsFrom(mounted)
	if len(recorded) != 2 {
		t.Fatalf("recorded %d datasets, want 2", len(recorded))
	}

	// The name and bucket travel with the identifier: a dataset renamed,
	// moved or deleted since must still mean something to a reader.
	if recorded[0].Name != "hds-essilor" || recorded[0].Bucket != "hds-essilor.wp2" {
		t.Errorf("the mount was not recorded as it was: %+v", recorded[0])
	}
	// The prefix is part of what was read: a dataset scoped to a prefix is
	// narrower than the dataset.
	if recorded[0].Prefix != "mri_test" {
		t.Errorf("the prefix was dropped: %q", recorded[0].Prefix)
	}
	// Writable or not is evidence of a different kind.
	if !recorded[0].ReadOnly || recorded[1].ReadOnly {
		t.Error("read-only was not recorded per mount")
	}
}

// A run with nothing attached records an empty list rather than nothing, so a
// reader can tell "read no data" from "we did not write it down".
func TestAJobWithNoDatasetsRecordsAnEmptyList(t *testing.T) {
	recorded := jobDatasetsFrom(nil)
	if recorded == nil {
		t.Fatal("a job with no mounts recorded nil, which reads as unknown")
	}
	if len(recorded) != 0 {
		t.Errorf("recorded %d datasets for a job with none", len(recorded))
	}
}
