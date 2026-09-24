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
	// Usage summarises what happened over a period.
	//
	// Aggregated in the store because the table is large - one installation
	// holds 690,000 events, of which 685,000 are a single ontology import
	// written row by row - and because that number is the reason the shape of
	// this report is what it is. See UsageReport.
	Usage(since, until time.Time) (UsageReport, error)
}

// UsageReport answers "who used this platform, doing what, and when".
//
// The daily series counts *people*, not events, and that is the load-bearing
// decision. A bulk import can write hundreds of thousands of audit rows in an
// afternoon; counted as events it buries a week of real work under one machine
// operation, and the chart then describes the import rather than the platform.
// Counted as distinct people per day, the same import is one person on one day,
// which is exactly what it was. The import is still visible - it appears in
// Actions with its real count - it simply no longer decides the shape.
//
// Nothing is filtered out. An exclusion list would need curating, and a
// curated list of "uninteresting" actions is a list that goes stale silently
// and starts hiding things nobody chose to hide.
type UsageReport struct {
	// Since and Until are the period asked for.
	Since time.Time
	Until time.Time
	// CoversSince and CoversUntil are the period the audit actually holds.
	//
	// Reported separately because they are often not the same thing: a report
	// over ninety days on an installation whose records begin twelve days ago
	// is not a quiet quarter, and a screen that does not say so invites exactly
	// that reading (ADR-034).
	CoversSince time.Time
	CoversUntil time.Time
	TotalEvents int
	People      []UsageActor
	Actions     []UsageAction
	Daily       []UsageDay
}

// UsageActor is one account's activity over the period.
type UsageActor struct {
	Actor    string
	Events   int
	LastSeen time.Time
}

// UsageAction is one kind of action and how often it happened.
type UsageAction struct {
	Action string
	Count  int
}

// UsageDay is one day's activity: how many distinct people, and how many
// events they produced.
type UsageDay struct {
	Day    time.Time
	People int
	Events int
}

// LastSeen is what an account has actually done, as opposed to what it is
// allowed to do.
type LastSeen struct {
	// First is the earliest successful sign-in the retention still holds.
	// With At it turns "seen" into a span, and it is what separates an
	// account that never signed in from one that did once, long ago - the
	// two rows an access review most wants told apart. Zero when never.
	First time.Time
	// At is the most recent successful sign-in. Zero means the account has
	// never signed in - a real answer, and usually the interesting one.
	At time.Time
	// Count is how many times it has, over whatever the audit retention keeps.
	// Reported alongside the date because one visit and two hundred are
	// different facts about the same last-seen date.
	Count int
}
