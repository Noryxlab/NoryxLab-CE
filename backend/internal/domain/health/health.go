// Package health holds what the platform noticed about itself.
//
// Distinct from the audit trail, which records what a *person* did. An
// operator asking "why did nobody tell me the backups stopped" is not asking
// who ran what; they are asking what the platform observed and whether anyone
// was told.
package health

import "time"

// Scope separates who an alert is for.
//
// A failed job is the user's problem and belongs on their screen, with the
// context that explains it. Interleaving those with "no backup for two days"
// buries the condition that costs the business everything under the ones that
// cost one person an afternoon - and on a busy platform, buries it completely.
type Scope string

const (
	// ScopePlatform is a technical condition an operator must act on.
	ScopePlatform Scope = "platform"
	// ScopeUser is a workload condition belonging to whoever owns it.
	ScopeUser Scope = "user"
)

// Severity mirrors the report's own levels.
type Severity string

const (
	SeverityInfo     Severity = "info"
	SeverityWarning  Severity = "warning"
	SeverityCritical Severity = "critical"
)

// Event is one condition, from the moment it was observed to the moment it
// cleared. Open events have no ResolvedAt.
//
// Stored as an interval rather than as two rows, because the question is "how
// long were we exposed", and reconstructing that by pairing a raise with a
// clear is the kind of join that silently loses the unmatched ones.
type Event struct {
	ID         string     `json:"id"`
	Key        string     `json:"key"`
	Source     string     `json:"source"`
	Severity   Severity   `json:"severity"`
	Summary    string     `json:"summary"`
	Detail     string     `json:"detail,omitempty"`
	RaisedAt   time.Time  `json:"raisedAt"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
}

// Open reports whether the condition is still current.
func (e Event) Open() bool { return e.ResolvedAt == nil }

// Duration is how long the condition lasted, or has lasted so far.
func (e Event) Duration(now time.Time) time.Duration {
	if e.ResolvedAt != nil {
		return e.ResolvedAt.Sub(e.RaisedAt)
	}
	return now.Sub(e.RaisedAt)
}
