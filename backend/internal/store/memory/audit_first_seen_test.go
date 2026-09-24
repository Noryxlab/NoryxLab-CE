package memory

import (
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/audit"
)

// Seen is a span, not a point: the earliest sign-in beside the latest.
//
// It is what separates an account that never signed in from one that did
// once, long ago - the two rows an access review most wants told apart, and
// which a last-seen date alone renders identically when both are old.
func TestLastSeenCarriesTheFirstSignIn(t *testing.T) {
	s := NewAuditStore()
	day := func(d int) time.Time { return time.Date(2026, 9, d, 9, 0, 0, 0, time.UTC) }
	for _, e := range []audit.Event{
		{ActorUserID: "alice", Action: "auth.login", Outcome: "success", OccurredAt: day(20)},
		{ActorUserID: "alice", Action: "auth.login", Outcome: "success", OccurredAt: day(3)},
		{ActorUserID: "alice", Action: "auth.login", Outcome: "success", OccurredAt: day(12)},
		{ActorUserID: "alice", Action: "auth.login", Outcome: "failure", OccurredAt: day(1)},
		{ActorUserID: "bob", Action: "project.created", Outcome: "success", OccurredAt: day(2)},
	} {
		if err := s.Create(e); err != nil {
			t.Fatal(err)
		}
	}
	seen, err := s.LastSeen()
	if err != nil {
		t.Fatal(err)
	}
	alice := seen["alice"]
	if !alice.First.Equal(day(3)) || !alice.At.Equal(day(20)) || alice.Count != 3 {
		t.Fatalf("alice = first %v, last %v, count %d", alice.First, alice.At, alice.Count)
	}
	// A failed login is not a sign-in, and an unrelated action is not one either.
	if _, ok := seen["bob"]; ok {
		t.Fatal("bob never signed in and is reported as seen")
	}
}
