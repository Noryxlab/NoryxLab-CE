package memory

import (
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/audit"
)

// Who actually uses the platform, as opposed to who is allowed to.
//
// The administration screen listed accounts with no way to tell a colleague
// from a leftover. The platform had recorded every sign-in since the beginning;
// nothing read them back.
func TestLastSeenKeepsTheMostRecentSignIn(t *testing.T) {
	store := NewAuditStore()
	early := time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)
	late := time.Date(2026, 9, 16, 18, 0, 0, 0, time.UTC)
	for _, at := range []time.Time{late, early} { // deliberately out of order
		if err := store.Create(audit.Event{
			ActorUserID: "amandine", Action: "auth.login", Outcome: "success", OccurredAt: at,
		}); err != nil {
			t.Fatal(err)
		}
	}

	seen, err := store.LastSeen()
	if err != nil {
		t.Fatal(err)
	}
	entry := seen["amandine"]
	if !entry.At.Equal(late) {
		t.Errorf("last seen %s, want %s", entry.At, late)
	}
	if entry.Count != 2 {
		t.Errorf("counted %d sign-ins, want 2", entry.Count)
	}
}

func TestOnlySuccessfulSignInsCount(t *testing.T) {
	// A run of failures is worth knowing about, but it is not evidence that
	// somebody is using the platform. Counting it as "last seen" would say the
	// opposite of what happened - that the account works.
	store := NewAuditStore()
	at := time.Date(2026, 9, 16, 18, 0, 0, 0, time.UTC)
	_ = store.Create(audit.Event{ActorUserID: "thomas", Action: "auth.login", Outcome: "failure", OccurredAt: at})

	seen, _ := store.LastSeen()
	if entry, ok := seen["thomas"]; ok && !entry.At.IsZero() {
		t.Errorf("a failed sign-in was reported as last seen: %v", entry)
	}
}

func TestAnAccountThatNeverSignedInIsAbsent(t *testing.T) {
	// Absent rather than zero-valued: the screen says "never signed in", which
	// is a real answer and usually the interesting one.
	store := NewAuditStore()
	_ = store.Create(audit.Event{ActorUserID: "stef", Action: "auth.login", Outcome: "success", OccurredAt: time.Now()})

	seen, _ := store.LastSeen()
	if _, ok := seen["someone-else"]; ok {
		t.Error("an account with no sign-in appeared in the map")
	}
}

func TestOtherActionsAreNotSignIns(t *testing.T) {
	// The audit holds everything the platform does. Only the sign-in answers
	// this question.
	store := NewAuditStore()
	_ = store.Create(audit.Event{ActorUserID: "cedric", Action: "project.create", Outcome: "success", OccurredAt: time.Now()})

	seen, _ := store.LastSeen()
	if _, ok := seen["cedric"]; ok {
		t.Error("a project creation was counted as a sign-in")
	}
}
