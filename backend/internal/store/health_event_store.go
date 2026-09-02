package store

import (
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/health"
)

// HealthEventStore keeps the platform's own conditions.
//
// It is deliberately small. This is not a log: a healthy platform writes a
// handful of rows a month, and a sick one writes one row per condition rather
// than one per observation.
type HealthEventStore interface {
	// Raise records a condition starting. Recording one that is already open
	// must not create a second row: the watcher re-raises after a restart,
	// having forgotten what it had already announced.
	Raise(event health.Event) error
	// Resolve closes the open event for a key. Unknown or already-closed keys
	// are not an error - a condition that cleared while the process was down
	// has nothing to close.
	Resolve(key string, at time.Time) error
	// Open lists the conditions still current, which is how a restarting
	// watcher recovers what it already announced.
	Open() ([]health.Event, error)
	// List returns events raised at or after since, newest first, capped.
	List(since time.Time, limit int) ([]health.Event, error)
	// Purge deletes resolved events older than before.
	Purge(before time.Time) (int, error)
}
