package keycloak

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Correcting an account, asked for by the Fondation Rothschild: a name typed
// wrong at creation had no remedy but deleting the account and making it
// again.
//
// The test that matters is what the request does *not* carry. Noryx identifies
// a person by their username, so a rename would leave their project roles,
// team memberships and audit events pointing at nobody. The refusal has to be
// in the payload, not only in the documentation.
func profileStub(t *testing.T, capture func(method, path string, body []byte)) *Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/protocol/openid-connect/token"):
			_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "t", "expires_in": 300})
		case strings.HasSuffix(r.URL.Path, "/users") && r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode([]User{
				{ID: "5f2a1c3d-0000-4000-8000-000000000001", Username: "andrew", Email: "andrew@for.fr"},
			})
		default:
			body, _ := io.ReadAll(r.Body)
			capture(r.Method, r.URL.Path, body)
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

func TestUpdatingAProfileNeverRenamesTheUsername(t *testing.T) {
	var method, path string
	var body []byte
	client := profileStub(t, func(m, p string, b []byte) { method, path, body = m, p, b })

	if err := client.UpdateUserProfile("andrew", "Andrew", "Eap", "andrew.eap@for.fr"); err != nil {
		t.Fatalf("profile update failed: %v", err)
	}
	if method != http.MethodPut {
		t.Fatalf("expected PUT, got %s", method)
	}
	// Reached by the identifier Keycloak knows, resolved from the username the
	// caller held.
	if !strings.HasSuffix(path, "/users/5f2a1c3d-0000-4000-8000-000000000001") {
		t.Fatalf("update went to %s", path)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if _, renamed := payload["username"]; renamed {
		t.Fatal("the update carries a username; every grant and audit event keyed on the old one would be orphaned")
	}
	// Nor anything else that would change what the account can do.
	for _, forbidden := range []string{"enabled", "credentials", "realmRoles", "groups", "requiredActions"} {
		if _, present := payload[forbidden]; present {
			t.Fatalf("the update carries %q, which is not a name correction", forbidden)
		}
	}
	if payload["firstName"] != "Andrew" || payload["lastName"] != "Eap" || payload["email"] != "andrew.eap@for.fr" {
		t.Fatalf("payload = %v", payload)
	}
}

// Clearing a field somebody filled in by mistake is as much a correction as
// changing it, so an empty value has to travel rather than be omitted.
func TestUpdatingAProfileCanClearAField(t *testing.T) {
	var body []byte
	client := profileStub(t, func(_, _ string, b []byte) { body = b })

	if err := client.UpdateUserProfile("andrew", "Andrew", "", "andrew@for.fr"); err != nil {
		t.Fatalf("profile update failed: %v", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	value, present := payload["lastName"]
	if !present || value != "" {
		t.Fatalf("an emptied field was not sent: lastName = %v (present=%v)", value, present)
	}
}
