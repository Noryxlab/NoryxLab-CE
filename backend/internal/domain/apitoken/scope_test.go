package apitoken

import (
	"net/http"
	"testing"
)

// A component's credential reaches its own work and stops there.
//
// Every component used to share one secret carrying the global administrator
// role: the backup runner held a credential that could delete a project, and
// revoking it for one component broke the others. A scoped component token is
// the remedy, and it is only a remedy if the scope actually refuses.
func TestOperateScopeReachesPlatformOperationsAndNothingElse(t *testing.T) {
	operate := []string{string(ScopeOperate)}

	for _, allowed := range []string{
		"/api/v1/admin/backups",
		"/api/v1/admin/backups/run",
		"/api/v1/admin/restore/rehearsal",
		"/api/v1/admin/health",
	} {
		if !Permits(operate, http.MethodPost, allowed) {
			t.Fatalf("a component that runs backups cannot POST %s", allowed)
		}
	}

	for _, refused := range []string{
		"/api/v1/projects",
		"/api/v1/datasets/abc",
		"/api/v1/workspaces",
		"/api/v1/admin/users",
	} {
		if Permits(operate, http.MethodDelete, refused) {
			t.Fatalf("a backup credential may DELETE %s: the scope refuses nothing", refused)
		}
	}

	// Reading stays open, as for every other non-invoke scope: a component
	// that cannot read the result of what it just ran is a component whose
	// operator gives it the full scope instead.
	if !Permits(operate, http.MethodGet, "/api/v1/projects") {
		t.Fatal("an operate token must still be able to read")
	}
}

// The full scope is what the shared secret was, and it stays available - named
// and revocable alone, which the shared secret never was.
func TestFullScopeStillAllowsEverything(t *testing.T) {
	if !Permits([]string{string(ScopeFull)}, http.MethodDelete, "/api/v1/projects/x") {
		t.Fatal("the full scope must keep allowing everything")
	}
}
