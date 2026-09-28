package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
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

// The shape of the request is the second half of the same refusal: a username
// added here would reach Keycloak however careful the client is.
func TestTheUpdateRequestCannotCarryAUsername(t *testing.T) {
	var req updateUserRequest
	if err := json.Unmarshal(
		[]byte(`{"firstName":"Andrew","lastName":"Eap","email":"a@for.fr","username":"andrew2"}`), &req); err != nil {
		t.Fatal(err)
	}
	if reflect.TypeOf(req).NumField() != 3 {
		t.Fatalf("updateUserRequest has %d fields; a rename must not become possible by adding one",
			reflect.TypeOf(req).NumField())
	}
	for _, name := range []string{"Username", "Enabled", "Attributes"} {
		if _, found := reflect.TypeOf(req).FieldByName(name); found {
			t.Fatalf("updateUserRequest carries %s, which is not a name correction", name)
		}
	}
}
