package memory

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/agent"
)

// The organisation is a set of statements. Writing the same line twice is the
// same line, or "who may ask whom" answers differently depending on how it was
// counted.
func TestWritingTheSameMandateTwiceLeavesOneLine(t *testing.T) {
	store := NewAgentTeamStore()
	for range 3 {
		if err := store.CreateMandate(agent.NewMandate("ops", "lead", "member", agent.ActionRestartApp, "stef")); err != nil {
			t.Fatal(err)
		}
	}
	mandates, err := store.ListMandates("ops")
	if err != nil {
		t.Fatal(err)
	}
	if len(mandates) != 1 {
		t.Fatalf("the same statement produced %d lines", len(mandates))
	}
}

// Deleting a team releases its agents rather than removing them; the same
// principle says a team's mandates are its own and not another team's.
func TestMandatesBelongToOneTeam(t *testing.T) {
	store := NewAgentTeamStore()
	_ = store.CreateMandate(agent.NewMandate("ops", "lead", "member", agent.ActionRestartApp, "stef"))
	_ = store.CreateMandate(agent.NewMandate("research", "lead2", "member2", agent.ActionRestartApp, "stef"))

	ops, _ := store.ListMandates("ops")
	if len(ops) != 1 || ops[0].TeamID != "ops" {
		t.Errorf("a team saw %d mandates, and not only its own", len(ops))
	}
}
