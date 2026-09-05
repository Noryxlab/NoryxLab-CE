package handlers

import (
	"net/http"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/apitoken"
)

// What a scoped token may do, stated as a table rather than trusted to the
// reading of a switch statement. These are the cases a security review asks
// about: can the CI token that runs a job also delete a project?
func TestScopesAllowWhatTheyName(t *testing.T) {
	cases := []struct {
		name    string
		scopes  []string
		method  string
		path    string
		allowed bool
	}{
		{"a read token reads", []string{"read"}, http.MethodGet, "/api/v1/projects", true},
		{"a read token does not write", []string{"read"}, http.MethodPost, "/api/v1/projects", false},
		{"a read token does not delete", []string{"read"}, http.MethodDelete, "/api/v1/projects/p1", false},
		{"a jobs token runs a job", []string{"jobs"}, http.MethodPost, "/api/v1/jobs", true},
		{"a jobs token runs a build", []string{"jobs"}, http.MethodPost, "/api/v1/builds", true},
		{"a jobs token cannot delete a project", []string{"jobs"}, http.MethodDelete, "/api/v1/projects/p1", false},
		{"a jobs token cannot launch a workspace", []string{"jobs"}, http.MethodPost, "/api/v1/workspaces", false},
		{"a workspaces token launches one", []string{"workspaces"}, http.MethodPost, "/api/v1/workspaces", true},
		{"a workspaces token stops one", []string{"workspaces"}, http.MethodDelete, "/api/v1/workspaces/w1", true},
		{"a workspaces token cannot run a job", []string{"workspaces"}, http.MethodPost, "/api/v1/jobs", false},
		{"combined scopes add up", []string{"workspaces", "jobs"}, http.MethodPost, "/api/v1/jobs", true},
		{"full is what tokens were before", []string{"full"}, http.MethodDelete, "/api/v1/projects/p1", true},
		// An existing token has no scopes stored, and must keep working
		// exactly as it did: an upgrade that breaks a running pipeline is a
		// worse outcome than an unscoped token.
		{"no scopes means unrestricted", nil, http.MethodDelete, "/api/v1/projects/p1", true},
		// Reading is allowed by every scope: a token that can launch a job but
		// not read its result would push its owner back to an unrestricted one.
		{"every scope may read", []string{"jobs"}, http.MethodGet, "/api/v1/projects", true},
		// Nothing under an unnamed family is allowed by a narrow scope, and
		// admin endpoints are exactly that.
		{"a narrow token reaches no admin endpoint", []string{"jobs"}, http.MethodPost, "/api/v1/admin/users", false},
		{"a prefix is not a path", []string{"workspaces"}, http.MethodPost, "/api/v1/workspaces-secret", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := apitoken.Permits(testCase.scopes, testCase.method, testCase.path); got != testCase.allowed {
				t.Errorf("%s %s with scopes %v = %v, want %v", testCase.method, testCase.path, testCase.scopes, got, testCase.allowed)
			}
		})
	}
}

func TestARefusalNamesTheScopeItWouldHaveNeeded(t *testing.T) {
	for _, testCase := range []struct {
		method string
		path   string
		want   string
	}{
		{http.MethodPost, "/api/v1/workspaces", "workspaces"},
		{http.MethodPost, "/api/v1/jobs", "jobs"},
		{http.MethodDelete, "/api/v1/projects/p1", "full"},
		{http.MethodGet, "/api/v1/projects", "read"},
	} {
		if got := apitoken.Explain(testCase.method, testCase.path); got != testCase.want {
			t.Errorf("%s %s would need %q, got %q", testCase.method, testCase.path, testCase.want, got)
		}
	}
}

func TestUnknownScopesAreRefusedRatherThanDropped(t *testing.T) {
	if apitoken.ValidScope("everything") {
		t.Error("an invented scope must not validate")
	}
	for _, scope := range apitoken.AllScopes() {
		if !apitoken.ValidScope(string(scope)) {
			t.Errorf("%q is offered by the platform and must validate", scope)
		}
	}
	// An empty request means unrestricted, not "no rights": a token created
	// with no scopes at all would otherwise be useless and silently so.
	if got := apitoken.NormalizeScopes(nil); len(got) != 1 || got[0] != string(apitoken.ScopeFull) {
		t.Errorf("an empty scope set must mean full, got %v", got)
	}
	if got := apitoken.NormalizeScopes([]string{"read", " read ", "jobs"}); len(got) != 2 {
		t.Errorf("scopes must be trimmed and deduplicated, got %v", got)
	}
}
