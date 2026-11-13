package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// The screen showed one person - the owner - because the API had no way to
// answer the question. Three people held a grant on the project Cédric
// reported; two of them were invisible, and one of those was the account
// nobody could explain.
func TestTheMembersListNamesEverybodyWithAGrant(t *testing.T) {
	projects := memory.NewProjectStore()
	owner := project.NewOwned("cyril", "test project", "")
	if err := projects.Create(owner); err != nil {
		t.Fatalf("creating the project: %v", err)
	}
	grants := memory.NewAccessStore()
	grants.SetRole(owner.ID, "carl", access.RoleViewer)
	grants.SetRole(owner.ID, "folignoc_test", access.RoleEditor)
	// A grant revoked the old way: an empty role, row left behind. It must not
	// be shown as membership of nothing.
	grants.SetRole(owner.ID, "parti", "")
	h := Handlers{projectStore: projects, accessStore: grants, authMode: "header"}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+owner.ID+"/members", nil)
	req.SetPathValue("projectID", owner.ID)
	req.Header.Set(userHeader, "cyril")
	h.ListProjectMembers(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	var payload struct{ Items []projectMember }
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decoding: %v", err)
	}
	roles := map[string]string{}
	for _, member := range payload.Items {
		roles[member.UserID] = member.Role
	}
	for user, want := range map[string]string{"cyril": "admin", "carl": "viewer", "folignoc_test": "editor"} {
		if roles[user] != want {
			t.Fatalf("%s = %q, want %q; the whole list was %v", user, roles[user], want, payload.Items)
		}
	}
	if _, shown := roles["parti"]; shown {
		t.Fatal("a grant revoked as an empty role is shown as membership")
	}
	if !payload.Items[0].Owner {
		t.Fatalf("the owner is not marked, or not first: %v", payload.Items)
	}
}

// Revoking used to mean writing an empty role, which left the row: the person
// reached nothing and still looked like a member.
func TestRemovingAMemberTakesTheGrantAway(t *testing.T) {
	projects := memory.NewProjectStore()
	owner := project.NewOwned("cyril", "test project", "")
	if err := projects.Create(owner); err != nil {
		t.Fatalf("creating the project: %v", err)
	}
	grants := memory.NewAccessStore()
	grants.SetRole(owner.ID, "folignoc_test", access.RoleEditor)
	h := Handlers{projectStore: projects, accessStore: grants, authMode: "header"}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+owner.ID+"/members/folignoc_test", nil)
	req.SetPathValue("projectID", owner.ID)
	req.SetPathValue("userID", "folignoc_test")
	req.Header.Set(userHeader, "cyril")
	h.DeleteProjectMember(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body %s", rec.Code, rec.Body.String())
	}
	if role, found := grants.GetRole(owner.ID, "folignoc_test"); found && string(role) != "" {
		t.Fatalf("the grant survived the removal: %q", role)
	}
}

// The owner's access does not come from that table, so removing a row would
// report success and change nothing.
func TestTheOwnerIsNotRemovedThroughTheMembersRoute(t *testing.T) {
	projects := memory.NewProjectStore()
	owner := project.NewOwned("cyril", "test project", "")
	if err := projects.Create(owner); err != nil {
		t.Fatalf("creating the project: %v", err)
	}
	grants := memory.NewAccessStore()
	h := Handlers{projectStore: projects, accessStore: grants, authMode: "header"}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+owner.ID+"/members/cyril", nil)
	req.SetPathValue("projectID", owner.ID)
	req.SetPathValue("userID", "cyril")
	req.Header.Set(userHeader, "cyril")
	h.DeleteProjectMember(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body %s", rec.Code, rec.Body.String())
	}
}
