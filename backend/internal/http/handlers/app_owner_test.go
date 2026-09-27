package handlers

import "testing"

// An application belongs to somebody who can still answer for it.
//
// Before ADR-039 it belonged to whoever launched it and to nobody else, so the
// question "an app belongs to FOR, its author leaves, who answers for it" had
// only one answer: name a successor, on the day somebody leaves. The owner
// pair lets that decision be made in advance - the organization that paid for
// it, or a service account that does not resign.
func TestOwnershipReadsTheEffectiveOwnerNotTheAuthor(t *testing.T) {
	// A legacy row: no pair, only the author. It is still personally owned.
	if !isPersonallyOwnedBy("", "claire", "claire") {
		t.Fatal("an app with no owner pair belongs to the person who launched it")
	}
	// Handed to an organization: no longer anybody's personal property, so a
	// departure must not try to transfer it.
	if isPersonallyOwnedBy("organization", "for", "claire") {
		t.Fatal("an app owned by an organization is not owned by a person")
	}
	// Handed to a service account: same answer, and that is the point - the
	// account is a user in the directory, but not the departing one.
	if isPersonallyOwnedBy("user", "for-production", "claire") {
		t.Fatal("an app owned by another account is not owned by this person")
	}
	if !isPersonallyOwnedBy("user", "claire", "CLAIRE") {
		t.Fatal("ownership is compared without regard to case, as everywhere else")
	}
}
