package apitoken

import (
	"net/http"
	"testing"
)

// A dataset token pushes data and does nothing else.
//
// The case it exists for: a partner organisation uploading imaging data into a
// bucket, unattended. Before this scope the only credential that could upload
// was ScopeFull, which could also delete the project it uploaded into.
func TestDatasetsScopeReachesDatasetsAndNothingElse(t *testing.T) {
	scopes := []string{string(ScopeDatasets)}

	for _, allowed := range []struct{ method, path string }{
		{http.MethodPut, "/api/v1/datasets/d1/objects/study/volume.dcm"},
		{http.MethodPost, "/api/v1/datasets/d1/folders"},
		{http.MethodPost, "/api/v1/datasets/d1/rename"},
		{http.MethodDelete, "/api/v1/datasets/d1/objects/old.dcm"},
	} {
		if !Permits(scopes, allowed.method, allowed.path) {
			t.Fatalf("%s %s was refused a dataset token", allowed.method, allowed.path)
		}
	}

	// The things it must not reach. Each one is why the scope exists: a
	// credential that uploads a study has no business doing any of these.
	for _, refused := range []struct{ method, path string }{
		{http.MethodPost, "/api/v1/workspaces"},
		{http.MethodDelete, "/api/v1/projects/p1"},
		{http.MethodPost, "/api/v1/jobs"},
		{http.MethodPost, "/api/v1/admin/backups"},
		{http.MethodDelete, "/api/v1/admin/users/someone"},
	} {
		if Permits(scopes, refused.method, refused.path) {
			t.Fatalf("%s %s was allowed by a dataset token", refused.method, refused.path)
		}
	}
}

// Reading stays allowed, like every scope but invoke.
//
// A pipeline that uploads and cannot then list what it uploaded would be
// pushed straight back to an unrestricted token, which is the outcome the
// whole model is avoiding.
func TestDatasetsScopeStillReads(t *testing.T) {
	scopes := []string{string(ScopeDatasets)}
	for _, path := range []string{"/api/v1/datasets", "/api/v1/projects", "/api/v1/workspaces"} {
		if !Permits(scopes, http.MethodGet, path) {
			t.Fatalf("GET %s was refused a dataset token", path)
		}
	}
}

// A refusal names the scope that would have worked.
//
// The pipeline on the other end gets "datasets" rather than "forbidden", which
// is the difference between a fix and a support thread. Before this, a refused
// upload was told it needed "full".
func TestDatasetRefusalNamesTheDatasetScope(t *testing.T) {
	if got := Explain(http.MethodPut, "/api/v1/datasets/d1/objects/x.dcm"); got != string(ScopeDatasets) {
		t.Fatalf("a refused upload was told it needed %q", got)
	}
}

// It is offered by the interface, between invoke and workspaces.
//
// The order is the contract of this list - least dangerous first - and a scope
// missing from it exists in the code and nowhere a person can choose it.
func TestDatasetsScopeIsOfferedInOrder(t *testing.T) {
	all := AllScopes()
	position := -1
	for i, scope := range all {
		if scope == ScopeDatasets {
			position = i
		}
	}
	if position < 0 {
		t.Fatal("the datasets scope is not offered by AllScopes")
	}
	if all[position-1] != ScopeInvoke || all[position+1] != ScopeWorkspaces {
		t.Fatalf("the datasets scope sits between %s and %s", all[position-1], all[position+1])
	}
	if !ValidScope("datasets") {
		t.Fatal("the datasets scope is refused at creation")
	}
}
