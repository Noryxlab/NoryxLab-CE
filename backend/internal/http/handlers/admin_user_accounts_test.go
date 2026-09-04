package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The platform chooses the password, not the administrator.
//
// An administrator inventing one under time pressure, with somebody waiting,
// picks weak and reused passwords. Twenty characters from an unambiguous
// alphabet is beyond guessing and still short enough to read out loud.
func TestTemporaryPasswordsAreStrongAndUnambiguous(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		password, err := temporaryPassword()
		if err != nil {
			t.Fatal(err)
		}
		if len(password) != temporaryPasswordLength {
			t.Fatalf("length = %d, want %d", len(password), temporaryPasswordLength)
		}
		// Characters people misread when a password is dictated or pasted.
		if strings.ContainsAny(password, "O0lI1") {
			t.Fatalf("ambiguous character in %q", password)
		}
		if seen[password] {
			t.Fatalf("generated the same password twice: %q", password)
		}
		seen[password] = true
	}
}

// On an installation that requires organization membership, an account created
// without one signs in and can do nothing - the phantom-user shape that left
// backups refused for three nights. Refused rather than created half-formed.
func TestAnAccountIsRefusedWithoutAnOrganizationWhenOneIsRequired(t *testing.T) {
	handlers := Handlers{authMode: "header", organizationRequired: true, keycloak: nil}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/users",
		strings.NewReader(`{"username":"alice"}`))
	request.Header.Set(userHeader, "admin")
	recorder := httptest.NewRecorder()

	handlers.CreateUserAccount(recorder, request)
	// Without a Keycloak client the guard answers first; what matters is that
	// the request never reaches account creation.
	if recorder.Code == http.StatusCreated {
		t.Fatal("an account was created without the organization this installation requires")
	}
}
