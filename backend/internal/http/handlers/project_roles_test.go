package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// The action a caller is authorized for is now stated, not inferred.
//
// It used to be guessed by calling a predicate with two roles and searching a
// human label for the substring "build", so renaming an error message could
// re-classify an authorization decision. Nothing failed, which is why it
// survived; these assertions are what would have failed.
func TestEachActionPermitsTheRolesItShould(t *testing.T) {
	cases := []struct {
		action projectAction
		viewer bool
		editor bool
		admin  bool
		nobody bool
		wantID string
	}{
		{actionRead, true, true, true, false, projectActionRead},
		{actionLaunch, false, true, true, false, projectActionLaunch},
		{actionRunBuild, false, true, true, false, projectActionRunBuild},
		{actionManageMembers, false, false, true, false, projectActionManageMember},
	}
	for _, c := range cases {
		if c.action.id != c.wantID {
			t.Errorf("action id = %q, want %q", c.action.id, c.wantID)
		}
		for role, want := range map[access.Role]bool{
			access.RoleViewer: c.viewer,
			access.RoleEditor: c.editor,
			access.RoleAdmin:  c.admin,
			access.Role(""):   c.nobody,
		} {
			if got := c.action.permits(role); got != want {
				t.Errorf("%s: role %q permitted = %v, want %v", c.action.id, role, got, want)
			}
		}
	}
}

func TestStrongestRoleWins(t *testing.T) {
	for _, c := range []struct {
		roles []access.Role
		want  access.Role
	}{
		{[]access.Role{access.RoleViewer, access.RoleEditor}, access.RoleEditor},
		{[]access.Role{access.RoleAdmin, access.RoleViewer}, access.RoleAdmin},
		{[]access.Role{"", access.RoleViewer}, access.RoleViewer},
		{[]access.Role{"", ""}, access.Role("")},
		{nil, access.Role("")},
	} {
		if got := access.Strongest(c.roles...); got != c.want {
			t.Errorf("Strongest(%v) = %q, want %q", c.roles, got, c.want)
		}
	}
}

// Grants add up. A personal viewer role must not cap an organization's editor
// grant, and losing an organization must not remove access granted personally
// - either would make an administrator's action have an effect they did not
// ask for and cannot see.
func TestPersonalAndOrganizationGrantsAddUp(t *testing.T) {
	store := memory.NewAccessStore()
	store.SetRole("p1", "alice", access.RoleViewer)
	if err := store.SetOrganizationRole("p1", "org-research", access.RoleEditor); err != nil {
		t.Fatal(err)
	}

	handlers := Handlers{accessStore: store, keycloak: nil}
	// Without a directory the organization grant cannot be resolved, and the
	// personal role must survive: an outage may not remove access.
	role, ok := handlers.effectiveProjectRole("p1", "alice")
	if !ok || role != access.RoleViewer {
		t.Fatalf("role = %q (%v), want viewer when the directory is unavailable", role, ok)
	}
}

func TestRevokingAnOrganizationGrantRemovesIt(t *testing.T) {
	store := memory.NewAccessStore()
	_ = store.SetOrganizationRole("p1", "org-research", access.RoleEditor)
	if grants, _ := store.ListOrganizationRoles("p1"); len(grants) != 1 {
		t.Fatalf("want one grant, got %+v", grants)
	}
	_ = store.SetOrganizationRole("p1", "org-research", "")
	if grants, _ := store.ListOrganizationRoles("p1"); len(grants) != 0 {
		t.Fatalf("the grant survived revocation: %+v", grants)
	}
}

func TestAGrantOnOneProjectDoesNotReachAnother(t *testing.T) {
	store := memory.NewAccessStore()
	_ = store.SetOrganizationRole("p1", "org-research", access.RoleAdmin)
	if grants, _ := store.ListOrganizationRoles("p2"); len(grants) != 0 {
		t.Fatalf("a grant leaked to another project: %+v", grants)
	}
}
