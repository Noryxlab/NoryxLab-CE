package handlers

import (
	"net/http"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/apitoken"
)

// A token is at most its owner, and the scope is what narrows it.
//
// The pair of properties this rests on: an administrator's token now carries
// the administrator role, and the scope gate can only refuse. Together they
// mean a read token in an administrator's hands reads what that administrator
// reads and writes nothing - which is what makes giving a token its owner's
// rights safe rather than reckless.
func TestReadScopeStillRefusesWrites(t *testing.T) {
	read := []string{string(apitoken.ScopeRead)}
	for _, path := range []string{"/api/v1/admin/component-tokens", "/api/v1/projects", "/api/v1/datasets"} {
		if apitoken.Permits(read, http.MethodDelete, path) {
			t.Fatalf("a read token must not DELETE %s", path)
		}
		if apitoken.Permits(read, http.MethodPost, path) {
			t.Fatalf("a read token must not POST %s", path)
		}
	}
}

// And reading is reading: the same token may GET what its owner may see. That
// is the half that makes the credential useful at all.
func TestReadScopeAllowsReads(t *testing.T) {
	read := []string{string(apitoken.ScopeRead)}
	for _, path := range []string{"/api/v1/projects", "/api/v1/admin/users"} {
		if !apitoken.Permits(read, http.MethodGet, path) {
			t.Fatalf("a read token must GET %s", path)
		}
	}
}
