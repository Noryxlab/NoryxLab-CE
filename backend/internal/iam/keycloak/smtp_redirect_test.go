package keycloak

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Where the person lands after choosing their password.
//
// Without a client and a redirect, Keycloak shows its own "your account has
// been updated" page: no link, no way back, and somebody who has never seen
// the platform has to be told its address separately. The invitation exists to
// bring them in, so it takes them there.
func TestTheInvitationCarriesTheClientAndTheReturnAddress(t *testing.T) {
	var asked string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/token") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"t","expires_in":300}`))
			return
		}
		asked = r.URL.RawQuery
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := New(Config{BaseURL: server.URL, Realm: "noryx", AdminRealm: "master", AdminUsername: "a", AdminPassword: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SendPasswordResetEmail("u1", 72*3600, "noryx-frontend", "https://example.org"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"client_id=noryx-frontend", "redirect_uri=https%3A%2F%2Fexample.org", "lifespan=259200"} {
		if !strings.Contains(asked, want) {
			t.Errorf("the request does not carry %s: %s", want, asked)
		}
	}
}

// A refusal must not cost the message.
//
// Keycloak rejects the whole call when the redirect is not registered on the
// client, and rejecting means no mail at all. A message that lands on a bare
// page is worse than one that lands in the platform, and far better than none.
func TestARejectedRedirectStillSendsTheMessage(t *testing.T) {
	var attempts []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/token") {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"access_token":"t","expires_in":300}`))
			return
		}
		attempts = append(attempts, r.URL.RawQuery)
		if strings.Contains(r.URL.RawQuery, "redirect_uri") {
			http.Error(w, `{"errorMessage":"Invalid redirect uri"}`, http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client, err := New(Config{BaseURL: server.URL, Realm: "noryx", AdminRealm: "master", AdminUsername: "a", AdminPassword: "b"})
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SendPasswordResetEmail("u1", 3600, "noryx-frontend", "https://not-registered.example"); err != nil {
		t.Fatalf("the message was not sent after the redirect was refused: %v", err)
	}
	if len(attempts) != 2 {
		t.Fatalf("expected a retry without the redirect, got %d attempt(s): %v", len(attempts), attempts)
	}
	if strings.Contains(attempts[1], "redirect_uri") {
		t.Error("the retry still carried the redirect that was just refused")
	}
	if !strings.Contains(attempts[1], "lifespan=3600") {
		t.Error("the retry dropped the lifespan along with the redirect")
	}
}
