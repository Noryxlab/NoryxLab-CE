package keycloak

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Membership by username, which is the only identifier callers hold.
//
// Keycloak identifies a member by its own id. Every screen and script that
// reaches for one holds a username, so the call went out with a name where an
// id belonged and came back 404 - reported as "no such organization", which
// named the one object that was certainly present. An administrator trying to
// empty an over-privileged organization was told that organization did not
// exist, with its members listed beside the message.
func keycloakStub(t *testing.T, record func(method, path string)) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/protocol/openid-connect/token"):
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "t", "expires_in": 300})
		case strings.HasSuffix(r.URL.Path, "/users"):
			_ = json.NewEncoder(w).Encode([]User{
				{ID: "5f2a1c3d-0000-4000-8000-000000000001", Username: "barantok", Email: "barantok@essilor.fr"},
				{ID: "5f2a1c3d-0000-4000-8000-000000000002", Username: "cedric"},
			})
		default:
			record(r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(server.Close)

	client, err := New(Config{
		BaseURL: server.URL, Realm: "noryx",
		AdminUsername: "admin", AdminPassword: "admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestRemovingAMemberByUsernameReachesTheirIdentifier(t *testing.T) {
	var method, path string
	client := keycloakStub(t, func(m, p string) { method, path = m, p })

	if err := client.RemoveOrganizationMember("essilor-org-id", "barantok"); err != nil {
		t.Fatalf("removal by username failed: %v", err)
	}
	if method != http.MethodDelete {
		t.Fatalf("expected DELETE, got %s", method)
	}
	if !strings.HasSuffix(path, "/members/5f2a1c3d-0000-4000-8000-000000000001") {
		t.Fatalf("the username was sent where the identifier belongs: %s", path)
	}
}

func TestAddingAMemberByEmailReachesTheirIdentifier(t *testing.T) {
	// The picker offers whatever it has; an email is as likely as a username.
	var path string
	client := keycloakStub(t, func(_, p string) { path = p })

	if err := client.AddOrganizationMember("essilor-org-id", "barantok@essilor.fr"); err != nil {
		t.Fatalf("addition by email failed: %v", err)
	}
	if !strings.HasSuffix(path, "/members") {
		t.Fatalf("unexpected path: %s", path)
	}
}

func TestAnIdentifierThatNamesNobodySaysSo(t *testing.T) {
	// And says it about the user, not about the organization. Sent onward as
	// an empty string it became a 404 from Keycloak, indistinguishable from a
	// missing organization once it crossed a handler.
	client := keycloakStub(t, func(string, string) {
		t.Error("an unresolvable name still reached Keycloak")
	})

	err := client.RemoveOrganizationMember("essilor-org-id", "quelquun-qui-nexiste-pas")
	if !IsNoSuchUser(err) {
		t.Fatalf("expected a no-such-user answer, got %v", err)
	}
}

func TestAnIdentifierThatIsAlreadyAnIdentifierIsLeftAlone(t *testing.T) {
	// A caller holding the real id must not need the directory to be readable.
	var path string
	client := keycloakStub(t, func(_, p string) { path = p })

	id := "5f2a1c3d-0000-4000-8000-000000000009"
	if err := client.RemoveOrganizationMember("essilor-org-id", id); err != nil {
		t.Fatalf("removal by identifier failed: %v", err)
	}
	if !strings.HasSuffix(path, "/members/"+id) {
		t.Fatalf("the identifier was rewritten: %s", path)
	}
}
