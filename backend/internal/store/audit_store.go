package store

import (
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/audit"
)

type AuditFilter struct {
	Since       *time.Time
	Until       *time.Time
	Action      string
	ActorUserID string
	ResourceID  string
	ProjectID   string
	Limit       int
}

type AuditStore interface {
	Create(event audit.Event) error
	List(filter AuditFilter) ([]audit.Event, error)
	// Stream visits every matching event in order, without holding them all in
	// memory. A backup of 690,000 events cannot be a slice: the platform would
	// spend a gigabyte to copy a file it is about to write out row by row.
	//
	// visit returning an error stops the walk and returns it, so a failed
	// upload does not keep reading a table for nothing.
	Stream(filter AuditFilter, visit func(audit.Event) error) error
	// LastSeen answers "when did this person last sign in", per account.
	//
	// The platform already records every sign-in; what it lacked was a way to
	// read the answer without walking the whole table. An administration screen
	// that lists accounts without saying which of them are actually used cannot
	// tell a colleague from a leftover, and leftovers are what access reviews
	// exist to find.
	//
	// Keyed by the actor as the audit records it, which is the username.
	LastSeen() (map[string]LastSeen, error)
}

// LastSeen is what an account has actually done, as opposed to what it is
// allowed to do.
type LastSeen struct {
	// At is the most recent successful sign-in. Zero means the account has
	// never signed in - a real answer, and usually the interesting one.
	At time.Time
	// Count is how many times it has, over whatever the audit retention keeps.
	// Reported alongside the date because one visit and two hundred are
	// different facts about the same last-seen date.
	Count int
}
