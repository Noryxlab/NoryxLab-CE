package handlers

import (
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
)

// The refusal has to name the account that blocks the deletion. It did not,
// and the case that produced the report is the one nothing else shows: an
// account that was disabled, and is therefore hidden from the screens an
// administrator would think to check, still holds its organization open.
func TestTheRefusalNamesTheDisabledMemberHoldingTheOrganizationOpen(t *testing.T) {
	members := []keycloak.User{
		{ID: "1", Username: "mf.compta3.and@gmail.com", Enabled: false},
	}
	message := blockingMembersMessage(memberLabels(members))
	if !strings.Contains(message, "mf.compta3.and@gmail.com") {
		t.Errorf("the refusal does not name the member: %s", message)
	}
	if !strings.Contains(message, "disabled") {
		t.Errorf("the refusal does not say the account is disabled, which is why it is invisible: %s", message)
	}
}

func TestAMemberWithoutAUsernameIsStillIdentified(t *testing.T) {
	names := memberLabels([]keycloak.User{{ID: "abc", Email: "someone@example.org", Enabled: true}})
	if names[0] != "someone@example.org" {
		t.Errorf("expected the email as the label, got %q", names[0])
	}
	names = memberLabels([]keycloak.User{{ID: "abc", Enabled: true}})
	if names[0] != "abc" {
		t.Errorf("a member with neither username nor email must still be identified, got %q", names[0])
	}
}

// A long list is truncated, but the count is the count: an administrator
// reading "5 member(s)" when there are forty would remove five and try again.
func TestALongMemberListKeepsItsRealCount(t *testing.T) {
	members := make([]keycloak.User, 12)
	for i := range members {
		members[i] = keycloak.User{ID: string(rune('a' + i)), Username: "user" + string(rune('a'+i)), Enabled: true}
	}
	message := blockingMembersMessage(memberLabels(members))
	if !strings.Contains(message, "12 member(s)") {
		t.Errorf("the count must be the real one: %s", message)
	}
	if !strings.Contains(message, "and 7 more") {
		t.Errorf("the list should be truncated with a remainder: %s", message)
	}
}
