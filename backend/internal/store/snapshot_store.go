package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/snapshot"

// SnapshotStore keeps what a dataset held at an instant.
//
// Entries are read back only to compare: nothing shows a manifest of a
// hundred thousand paths on a screen, and returning one through the API would
// disclose the shape of a regulated dataset to anybody allowed to list it.
type SnapshotStore interface {
	CreateSnapshot(item snapshot.Snapshot, entries []snapshot.Entry) error
	ListSnapshots(datasetID string) ([]snapshot.Snapshot, error)
	// GetSnapshotEntries reads one snapshot's manifest, checking it belongs to
	// the dataset named: a snapshot identifier from another dataset must not
	// become a way to read this one's contents.
	GetSnapshotEntries(snapshotID, datasetID string) ([]snapshot.Entry, bool, error)
	// LatestSnapshot is what the nightly scan compares against.
	LatestSnapshot(datasetID string) (snapshot.Snapshot, bool, error)
}
