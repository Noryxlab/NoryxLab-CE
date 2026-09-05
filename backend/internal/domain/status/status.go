// Package status is the vocabulary of states the platform publishes.
//
// There were two sources of truth for it. The backend invented status strings
// wherever it happened to set one, and the interface classified them with a
// regular expression written from memory. "launching" was in neither list: a
// workspace that had actually started showed as launching, the list stopped
// polling because the status did not read as pending, and the screen stayed
// that way until somebody reloaded.
//
// This package is the list. A status the backend can emit and that is not
// declared here fails a test, and the interface is checked against the same
// declarations, so the two cannot drift apart in silence again.
package status

// Kind says how a status should be read, and it is the part the interface
// needs: whether to keep polling, and whether the thing is usable.
type Kind string

const (
	// KindPending means work is in flight. The interface keeps polling.
	KindPending Kind = "pending"
	// KindSuccess means usable, or finished as intended.
	KindSuccess Kind = "success"
	// KindFailed means it will not become usable without intervention.
	KindFailed Kind = "failed"
	// KindDegraded means it finished without fulfilling its contract - an
	// incomplete backup, for instance. Neither a success nor a failure, and
	// never shown as either (ADR-034).
	KindDegraded Kind = "degraded"
	// KindStopped means deliberately not running. Not a failure.
	KindStopped Kind = "stopped"
	// KindUnknown means the platform has no answer yet - a check that has not
	// run, a resource it cannot see. Distinct from a failure on purpose: it
	// says nothing about the thing itself.
	KindUnknown Kind = "unknown"
)

// Vocabulary is every status the backend emits, and how to read it.
var Vocabulary = map[string]Kind{
	"submitted": KindPending,
	"pending":   KindPending,
	"launching": KindPending,
	"building":  KindPending,
	"running":   KindSuccess,
	"active":    KindSuccess,
	"available": KindSuccess,
	"succeeded": KindSuccess,
	"failed":    KindFailed,
	"unhealthy": KindFailed,
	"canceled":  KindFailed,
	"degraded":  KindDegraded,
	"stopped":   KindStopped,
	"unchecked": KindUnknown,
	"missing":   KindUnknown,
}

// Of returns how to read a status, and whether it is one the platform declares.
func Of(value string) (Kind, bool) {
	kind, declared := Vocabulary[value]
	return kind, declared
}

// Names lists the declared statuses. Order is not guaranteed; callers that
// need one sort it.
func Names() []string {
	names := make([]string, 0, len(Vocabulary))
	for name := range Vocabulary {
		names = append(names, name)
	}
	return names
}
