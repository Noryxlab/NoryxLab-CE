// Package snapshot is what a dataset held at an instant, written down.
//
// It copies nothing, and that is the whole design. An extract already freezes
// a selection without duplicating a byte, under a rule this package keeps:
// these are regulated datasets, and the platform's job is to provide the
// tools, not to make second copies of the evidence. A snapshot records what
// was there - path, size, last modification, ETag - and the bytes stay where
// they are.
//
// What that buys is the question nothing could answer before: "has this file
// changed since?". Neither the ontology nor the extract recorded a
// fingerprint, so a dataset could be rewritten under a finished study and
// nothing in the platform would notice. Comparing a snapshot with the bucket
// as it stands now is also what makes the nightly detection of writes made
// outside the API possible - approved on 2026-11-17 alongside the co-mount
// rule, and impossible to build without this.
//
// Copying a dataset is a separate act behind a separate right
// (platformrole.CopyDataset), because doubling regulated bytes creates a
// second place to govern, find and delete them.
package snapshot

import "time"

// Snapshot is one reading of a dataset.
type Snapshot struct {
	ID        string `json:"id"`
	DatasetID string `json:"datasetId"`
	// Label is what a person calls it: "before the November re-export".
	Label     string    `json:"label,omitempty"`
	TakenBy   string    `json:"takenBy"`
	TakenAt   time.Time `json:"takenAt"`
	Objects   int64     `json:"objects"`
	TotalSize int64     `json:"totalSize"`
	// CopyOf names the dataset this snapshot's bytes were copied into, empty
	// when nothing was copied - which is the normal case.
	CopyDatasetID string `json:"copyDatasetId,omitempty"`
}

// Entry is one object as it was.
type Entry struct {
	Path         string    `json:"path"`
	Size         int64     `json:"size"`
	ETag         string    `json:"etag"`
	LastModified time.Time `json:"lastModified"`
}

// Change is one difference between a snapshot and what is there now.
type Change struct {
	Path string `json:"path"`
	// Kind is added, removed or modified. Three words rather than a count:
	// "forty-one differences" sends somebody looking, "modified
	// SELENA-01-003/visit1/oct.e2e" tells them what happened.
	Kind string `json:"kind"`
	// Was and Now describe the object on each side, for a modification.
	Was *Entry `json:"was,omitempty"`
	Now *Entry `json:"now,omitempty"`
}

const (
	Added    = "added"
	Removed  = "removed"
	Modified = "modified"
)

// Compare says how a dataset differs from a snapshot of it.
//
// ETag first, size second. An ETag is not a portable checksum - a multipart
// upload produces a different one for the same bytes - so it can say "these
// differ" and must not be read as "these are identical". A size that matches
// with a different ETag is still reported: the honest answer is that
// something changed, not that nothing did.
func Compare(before, now []Entry) []Change {
	avant := make(map[string]Entry, len(before))
	for _, item := range before {
		avant[item.Path] = item
	}
	changes := []Change{}
	seen := make(map[string]bool, len(now))
	for _, item := range now {
		seen[item.Path] = true
		previous, known := avant[item.Path]
		if !known {
			entry := item
			changes = append(changes, Change{Path: item.Path, Kind: Added, Now: &entry})
			continue
		}
		if previous.ETag != item.ETag || previous.Size != item.Size {
			was, current := previous, item
			changes = append(changes, Change{Path: item.Path, Kind: Modified, Was: &was, Now: &current})
		}
	}
	for path, item := range avant {
		if !seen[path] {
			was := item
			changes = append(changes, Change{Path: path, Kind: Removed, Was: &was})
		}
	}
	return changes
}
