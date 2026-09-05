package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/apitoken"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/app"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Disabling an account is only safe if what it owned is handed on. A project
// whose owner is gone has nobody who can grant access to it; an app nobody can
// republish. These check the handover, not the Keycloak call.
func deactivationFixture(t *testing.T) Handlers {
	t.Helper()
	projects := memory.NewProjectStore()
	owned := project.NewOwned("leaver", "Their project", "")
	if err := projects.Create(owned); err != nil {
		t.Fatal(err)
	}
	shared := project.NewOwned("someone-else", "Not theirs", "")
	if err := projects.Create(shared); err != nil {
		t.Fatal(err)
	}
	byOrganization := project.NewOwned("leaver", "Owned by the team", "")
	byOrganization.OwnerType = "organization"
	byOrganization.OwnerID = "org-research"
	if err := projects.Create(byOrganization); err != nil {
		t.Fatal(err)
	}

	apps := memory.NewAppStore()
	if err := apps.Create(app.App{ID: "a1", Name: "Their app", OwnerUserID: "leaver", ProjectID: owned.ID}); err != nil {
		t.Fatal(err)
	}

	return Handlers{
		projectStore:  projects,
		appStore:      apps,
		apiTokenStore: memory.NewAPITokenStore(),
	}
}

func TestWhatAnAccountOwnsIsListedByName(t *testing.T) {
	h := deactivationFixture(t)

	owned, err := h.ownedBy("leaver")
	if err != nil {
		t.Fatal(err)
	}
	if len(owned.Projects) != 1 || owned.Projects[0] != "Their project" {
		t.Errorf("expected the one personally owned project, got %v", owned.Projects)
	}
	// A project owned by an organization already has somebody responsible for
	// it, which is the point of organization ownership.
	if strings.Join(owned.Projects, ",") == "Owned by the team" {
		t.Error("an organization's project must not be treated as personal property")
	}
	if len(owned.Apps) != 1 {
		t.Errorf("expected one app, got %v", owned.Apps)
	}
	if owned.count() != 2 {
		t.Errorf("count = %d, want 2", owned.count())
	}
}

func TestOwnershipMovesToTheSuccessorAndNothingElseMoves(t *testing.T) {
	h := deactivationFixture(t)

	moved, err := h.transferOwnership("leaver", "keeper")
	if err != nil {
		t.Fatal(err)
	}
	if moved.count() != 2 {
		t.Fatalf("expected two resources moved, got %d (%+v)", moved.count(), moved)
	}

	projects, err := h.projectStore.List()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range projects {
		switch item.Name {
		case "Their project":
			if item.OwnerID != "keeper" {
				t.Errorf("the successor must own it, got %q", item.OwnerID)
			}
		case "Not theirs":
			if item.OwnerID != "someone-else" {
				t.Errorf("somebody else's project must not move, got %q", item.OwnerID)
			}
		case "Owned by the team":
			if item.OwnerType != "organization" || item.OwnerID != "org-research" {
				t.Errorf("an organization's project must stay with the organization, got %s/%s", item.OwnerType, item.OwnerID)
			}
		}
	}

	// Nothing left behind: a second pass must find nothing to hand on.
	after, err := h.ownedBy("leaver")
	if err != nil {
		t.Fatal(err)
	}
	if after.count() != 0 {
		t.Errorf("the account still owns %d resource(s) after the transfer: %+v", after.count(), after)
	}
}

// The half of disabling that has nothing to do with the identity provider: a
// token acts as its owner, so an account disabled while its tokens still work
// is not disabled at all.
func TestDisablingRevokesTheAccountsTokens(t *testing.T) {
	h := deactivationFixture(t)
	now := time.Now().UTC()

	for _, id := range []string{"t1", "t2"} {
		if err := h.apiTokenStore.Put(apitoken.Token{ID: id, UserID: "leaver", Name: id, CreatedAt: now}); err != nil {
			t.Fatal(err)
		}
	}
	if err := h.apiTokenStore.Put(apitoken.Token{ID: "t3", UserID: "keeper", Name: "keeper's", CreatedAt: now}); err != nil {
		t.Fatal(err)
	}

	if revoked := h.revokeAllTokens("leaver"); revoked != 2 {
		t.Fatalf("expected two tokens revoked, got %d", revoked)
	}
	for _, id := range []string{"t1", "t2"} {
		token, found, err := h.apiTokenStore.Get(id)
		if err != nil || !found {
			t.Fatalf("token %s should still exist, revoked rather than deleted", id)
		}
		if token.Active(time.Now().UTC()) {
			t.Errorf("token %s is still usable after the account was disabled", id)
		}
	}
	// Somebody else's credentials are not collateral.
	other, found, err := h.apiTokenStore.Get("t3")
	if err != nil || !found || !other.Active(time.Now().UTC()) {
		t.Error("another user's token must be untouched")
	}
	// Revoking twice reports nothing new rather than double-counting.
	if again := h.revokeAllTokens("leaver"); again != 0 {
		t.Errorf("a second pass must revoke nothing, got %d", again)
	}
}
