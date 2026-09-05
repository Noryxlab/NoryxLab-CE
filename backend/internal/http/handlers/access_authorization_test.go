package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Authorization is the part of this package worth testing first: it decides who
// reaches what, it was rewritten this week, and every one of its rules was
// carried by comments rather than by anything that fails when broken.
//
// fakeKeycloak answers the two calls the authorization path makes - a token,
// then the organizations a user belongs to. It is a real HTTP server rather
// than a stubbed interface because `Handlers` holds a concrete *keycloak.Client:
// faking below that would test a different program.
func fakeKeycloak(t *testing.T, memberships map[string][]keycloak.Organization) *keycloak.Client {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/protocol/openid-connect/token"):
			writeTestJSON(t, w, map[string]any{"access_token": "test-token", "expires_in": 60})
		case strings.Contains(r.URL.Path, "/users"):
			// The client resolves a name to an id by listing every user, not by
			// querying one - so the fake has to answer with the whole
			// directory. A name absent from it resolves to no id at all, which
			// is how a stranger behaves.
			users := []map[string]any{}
			for name := range memberships {
				users = append(users, map[string]any{"id": name, "username": name, "enabled": true})
			}
			writeTestJSON(t, w, users)
		case strings.Contains(r.URL.Path, "/organizations/members/"):
			segments := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
			user := segments[len(segments)-2]
			organizations := memberships[user]
			if organizations == nil {
				organizations = []keycloak.Organization{}
			}
			writeTestJSON(t, w, organizations)
		default:
			writeTestJSON(t, w, []map[string]any{})
		}
	}))
	t.Cleanup(server.Close)

	client, err := keycloak.New(keycloak.Config{
		BaseURL:       server.URL,
		Realm:         "noryx",
		AdminRealm:    "master",
		AdminUsername: "admin",
		AdminPassword: "admin",
	})
	if err != nil {
		t.Fatalf("building the test keycloak client: %v", err)
	}
	return client
}

func writeTestJSON(t *testing.T, w http.ResponseWriter, payload any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		t.Fatalf("writing the fake keycloak response: %v", err)
	}
}

// The same rule as TestPersonalAndOrganizationGrantsAddUp, but through the
// whole path: the store, the directory, and the resolution between them. That
// test proves the arithmetic on roles; this one proves the user is actually
// found in the organization the grant was given to.
func TestOrganizationGrantsReachAUserThroughTheDirectory(t *testing.T) {
	projects := memory.NewProjectStore()
	item := project.NewOwned("owner", "Shared project", "")
	if err := projects.Create(item); err != nil {
		t.Fatal(err)
	}
	access1 := memory.NewAccessStore()
	access1.SetRole(item.ID, "reader", access.RoleViewer)
	if err := access1.SetOrganizationRole(item.ID, "org-research", access.RoleEditor); err != nil {
		t.Fatal(err)
	}

	h := Handlers{
		projectStore: projects,
		accessStore:  access1,
		keycloak: fakeKeycloak(t, map[string][]keycloak.Organization{
			"reader":   {{ID: "org-research", Name: "Research", Enabled: true}},
			"outsider": {{ID: "org-other", Name: "Other", Enabled: true}},
		}),
	}

	role, ok := h.effectiveProjectRole(item.ID, "reader")
	if !ok || role != access.RoleEditor {
		t.Fatalf("the organization's editor grant must win over a personal viewer role, got %q (ok=%v)", role, ok)
	}

	role, ok = h.effectiveProjectRole(item.ID, "outsider")
	if ok || role != "" {
		t.Fatalf("a member of an unrelated organization must hold no role, got %q (ok=%v)", role, ok)
	}
}

// A directory outage must not hand out access - and must not remove the access
// somebody holds personally either.
func TestDirectoryOutageNeitherGrantsNorRemovesAccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "keycloak is down", http.StatusBadGateway)
	}))
	t.Cleanup(server.Close)
	client, err := keycloak.New(keycloak.Config{BaseURL: server.URL, Realm: "noryx", AdminUsername: "a", AdminPassword: "b"})
	if err != nil {
		t.Fatal(err)
	}

	projects := memory.NewProjectStore()
	item := project.NewOwned("owner", "Project", "")
	if err := projects.Create(item); err != nil {
		t.Fatal(err)
	}
	store := memory.NewAccessStore()
	store.SetRole(item.ID, "member", access.RoleEditor)
	if err := store.SetOrganizationRole(item.ID, "org-research", access.RoleAdmin); err != nil {
		t.Fatal(err)
	}

	h := Handlers{projectStore: projects, accessStore: store, keycloak: client}

	role, ok := h.effectiveProjectRole(item.ID, "member")
	if !ok || role != access.RoleEditor {
		t.Fatalf("a personal grant must survive a directory outage, got %q (ok=%v)", role, ok)
	}
	if role, ok := h.effectiveProjectRole(item.ID, "stranger"); ok || role != "" {
		t.Fatalf("an outage must not grant the organization role to a stranger, got %q (ok=%v)", role, ok)
	}
}

// The owner of a project reaches it without any explicit grant, whether the
// owner is a person or an organization they belong to.
func TestOwnershipGrantsAccessWithoutAGrant(t *testing.T) {
	projects := memory.NewProjectStore()
	personal := project.NewOwned("stef", "Personal project", "")
	if err := projects.Create(personal); err != nil {
		t.Fatal(err)
	}
	organizationOwned := project.NewOwned("stef", "Team project", "")
	organizationOwned.OwnerType = "organization"
	organizationOwned.OwnerID = "org-research"
	if err := projects.Create(organizationOwned); err != nil {
		t.Fatal(err)
	}

	h := Handlers{
		projectStore: projects,
		accessStore:  memory.NewAccessStore(),
		keycloak: fakeKeycloak(t, map[string][]keycloak.Organization{
			"colleague": {{ID: "org-research", Name: "Research", Enabled: true}},
		}),
	}

	if !h.hasProjectMembership("stef", personal.ID) {
		t.Error("the owner must reach their own project")
	}
	if h.hasProjectMembership("colleague", personal.ID) {
		t.Error("a colleague must not reach a personally owned project without a grant")
	}
	if !h.hasProjectMembership("colleague", organizationOwned.ID) {
		t.Error("a member of the owning organization must reach the project")
	}
	if h.hasProjectMembership("stranger", organizationOwned.ID) {
		t.Error("a stranger must not reach an organization's project")
	}
}

// A disabled organization is not a membership. Keycloak keeps the row; the
// platform must not treat it as access.
func TestDisabledOrganizationGrantsNothing(t *testing.T) {
	projects := memory.NewProjectStore()
	item := project.NewOwned("owner", "Project", "")
	item.OwnerType = "organization"
	item.OwnerID = "org-closed"
	if err := projects.Create(item); err != nil {
		t.Fatal(err)
	}
	h := Handlers{
		projectStore: projects,
		accessStore:  memory.NewAccessStore(),
		keycloak: fakeKeycloak(t, map[string][]keycloak.Organization{
			"former": {{ID: "org-closed", Name: "Closed", Enabled: false}},
		}),
	}
	if h.userBelongsToOrganization("former", "org-closed") {
		t.Error("a disabled organization must not count as membership")
	}
	if h.hasProjectMembership("former", item.ID) {
		t.Error("a disabled organization must not carry project access")
	}
}

// The bootstrap administrator is the one identity that bypasses project rules.
// An empty configured name must not turn every anonymous caller into one.
func TestGlobalAdminIsNotTheEmptyString(t *testing.T) {
	h := Handlers{bootstrapAdminUser: "stef"}
	if !h.isGlobalAdminUserID("stef") || !h.isGlobalAdminUserID("STEF") {
		t.Error("the configured administrator must be recognised, whatever the case")
	}
	if h.isGlobalAdminUserID("") || h.isGlobalAdminUserID("someone") {
		t.Error("nobody else is the administrator")
	}
	empty := Handlers{}
	if empty.isGlobalAdminUserID("") || empty.isGlobalAdminUserID("stef") {
		t.Error("with no administrator configured, nobody is one")
	}
}
