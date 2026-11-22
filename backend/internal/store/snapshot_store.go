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
	// DeleteSnapshot removes one reading and its manifest.
	//
	// It has to exist, and not only to undo a mistake. A snapshot is how a
	// dataset is enrolled in the nightly drift check; with no way to remove
	// the last one, nothing could ever be un-enrolled, and a dataset
	// somebody snapshotted once would be listed and reported on for as long
	// as the installation lives. The dataset is named so an identifier from
	// another one cannot become a way to delete this one's readings.
	DeleteSnapshot(snapshotID, datasetID string) (bool, error)
}
