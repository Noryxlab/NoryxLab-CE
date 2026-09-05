package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/app"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Published applications are the one place where somebody outside the platform
// can be let in on purpose, which is exactly why every mode needs stating.
// This was the last untested proxy, and it has the same shape as the workspace
// one - where two invisible defects lived.
func appProxyFixture(t *testing.T, mode string, record *app.App) Handlers {
	t.Helper()
	projects := memory.NewProjectStore()
	item := project.NewOwned("owner", "App project", "")
	if err := projects.Create(item); err != nil {
		t.Fatal(err)
	}
	accessStore := memory.NewAccessStore()
	accessStore.SetRole(item.ID, "member", access.RoleEditor)

	record.ProjectID = item.ID
	record.AccessMode = mode

	return Handlers{
		projectStore: projects,
		accessStore:  accessStore,
		sessionStore: memory.NewSessionStore(),
		authMode:     "oidc",
		keycloak: fakeKeycloak(t, map[string][]keycloak.Organization{
			"colleague": {{ID: "org-research", Name: "Research", Enabled: true}},
			"outsider":  {{ID: "org-other", Name: "Other", Enabled: true}},
		}),
	}
}

func TestAPublishedAppIsReachableWithoutSigningIn(t *testing.T) {
	record := app.App{OwnerUserID: "owner"}
	h := appProxyFixture(t, "public", &record)

	recorder := httptest.NewRecorder()
	if !h.requireAppAccess(recorder, httptest.NewRequest(http.MethodGet, "/apps/x/", nil), record) {
		t.Fatalf("a public app must be reachable by anyone, got %d", recorder.Code)
	}
}

// A private app is the owner's alone. Not the project's members, not a
// colleague: the mode says private and the code must mean it.
func TestAPrivateAppIsTheOwnersAlone(t *testing.T) {
	record := app.App{OwnerUserID: "owner"}
	h := appProxyFixture(t, "private", &record)

	owner := withSession(t, h, "owner")
	request := httptest.NewRequest(http.MethodGet, "/apps/x/", nil)
	request.AddCookie(owner)
	if !h.requireAppAccess(httptest.NewRecorder(), request, record) {
		t.Error("the owner must reach their own private app")
	}

	other := withSession(t, h, "member")
	// Two sessions cannot share one token in this fixture, so the second is
	// created explicitly rather than reusing the first.
	request = httptest.NewRequest(http.MethodGet, "/apps/x/", nil)
	request.AddCookie(other)
	recorder := httptest.NewRecorder()
	if h.requireAppAccess(recorder, request, record) {
		t.Error("a project member must not reach a private app")
	}
	if recorder.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", recorder.Code)
	}
}

// An organization-scoped app admits the members of the organizations it names,
// and nobody else - including a member of a different organization, which is
// the case that a naive "has any organization" check would let through.
func TestAnOrganizationAppAdmitsItsMembersOnly(t *testing.T) {
	record := app.App{OwnerUserID: "owner", AllowedOrganizations: []string{"org-research"}}
	h := appProxyFixture(t, "organization", &record)

	request := httptest.NewRequest(http.MethodGet, "/apps/x/", nil)
	request.AddCookie(withSession(t, h, "colleague"))
	if !h.requireAppAccess(httptest.NewRecorder(), request, record) {
		t.Error("a member of the named organization must be admitted")
	}

	request = httptest.NewRequest(http.MethodGet, "/apps/x/", nil)
	request.AddCookie(withSession(t, h, "outsider"))
	recorder := httptest.NewRecorder()
	if h.requireAppAccess(recorder, request, record) {
		t.Error("a member of another organization must not be admitted")
	}
}

// A browser gets sent to sign in; a script gets an answer it can read. The
// difference matters: an API client following a redirect to a login page
// receives HTML and reports a parse error, which tells its author nothing.
func TestABrowserIsRedirectedAndAClientIsAnswered(t *testing.T) {
	record := app.App{OwnerUserID: "owner"}
	h := appProxyFixture(t, "private", &record)

	browser := httptest.NewRequest(http.MethodGet, "/apps/x/page", nil)
	browser.Header.Set("Accept", "text/html")
	recorder := httptest.NewRecorder()
	if h.requireAppAccess(recorder, browser, record) {
		t.Fatal("an anonymous visitor must not be admitted to a private app")
	}
	if recorder.Code != http.StatusFound {
		t.Errorf("a browser must be sent to sign in, got %d", recorder.Code)
	}
	if location := recorder.Header().Get("Location"); location == "" {
		t.Error("the redirect must say where to go")
	}

	client := httptest.NewRequest(http.MethodGet, "/apps/x/api", nil)
	client.Header.Set("Accept", "application/json")
	recorder = httptest.NewRecorder()
	if h.requireAppAccess(recorder, client, record) {
		t.Fatal("an anonymous client must not be admitted")
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("a client asking for JSON must get 401, got %d", recorder.Code)
	}
}

// The redirect target comes from the request, so it is the same open-redirect
// question as the login flow - and the answer has to be the same.
func TestTheLoginRedirectNeverLeavesThePlatform(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/apps/x/page?next=1", nil)
	recorder := httptest.NewRecorder()
	redirectToInteractiveLogin(recorder, request)

	location := recorder.Header().Get("Location")
	if location == "" || location[0] != '/' {
		t.Fatalf("the redirect must stay on this platform, got %q", location)
	}
}
