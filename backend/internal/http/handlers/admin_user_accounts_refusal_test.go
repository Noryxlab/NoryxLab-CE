package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
)

// A refused field reaches the person, because it is the one part of the
// directory's answer they can act on.
//
// Measured on 2026-11-11: creating "Chuck Noryx" was refused twice for the
// space in the username, Keycloak said exactly that, and the screen said
// "failed to create the account" - which reads as a permission problem or an
// outage, and sends somebody looking in the wrong place.
func TestARefusedFieldIsNamedToTheCaller(t *testing.T) {
	err := &keycloak.APIError{
		Path:       "users",
		StatusCode: http.StatusBadRequest,
		Body:       `{"field":"username","errorMessage":"error-username-invalid-character","params":["username"]}`,
	}

	recorder := httptest.NewRecorder()
	Handlers{}.writeKeycloakUserError(recorder, "create the account", err)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("a refused field is the caller's fault, not a gateway failure: %d", recorder.Code)
	}
	body := recorder.Body.String()
	if !strings.Contains(body, "username cannot contain spaces") {
		t.Errorf("the answer should say what to change, got %q", body)
	}
	if !strings.Contains(body, `"field":"username"`) {
		t.Errorf("the answer should name the field so a form can point at it, got %q", body)
	}
}

// What is not a field refusal keeps its old shape: a bad gateway, with the
// directory's internals left in the log.
func TestAnOutageIsStillAGatewayFailure(t *testing.T) {
	err := &keycloak.APIError{Path: "users", StatusCode: http.StatusInternalServerError, Body: "upstream exploded"}

	recorder := httptest.NewRecorder()
	Handlers{}.writeKeycloakUserError(recorder, "create the account", err)

	if recorder.Code != http.StatusBadGateway {
		t.Fatalf("expected a gateway failure, got %d", recorder.Code)
	}
	if strings.Contains(recorder.Body.String(), "upstream exploded") {
		t.Error("the directory's own body must not cross the boundary")
	}
}

// And a conflict keeps the sentence it already had, rather than becoming a
// field refusal with a code nobody reads.
func TestAnExistingUserKeepsItsOwnSentence(t *testing.T) {
	err := &keycloak.APIError{Path: "users", StatusCode: http.StatusConflict, Body: `{"errorMessage":"User exists with same username"}`}

	recorder := httptest.NewRecorder()
	Handlers{}.writeKeycloakUserError(recorder, "create the account", err)

	if recorder.Code != http.StatusConflict {
		t.Fatalf("expected a conflict, got %d", recorder.Code)
	}
	if !strings.Contains(recorder.Body.String(), "already exists") {
		t.Error("the conflict should still say the name is taken")
	}
}

func TestRefusalMessagesNameTheField(t *testing.T) {
	for _, item := range []struct {
		refusal keycloak.FieldRefusal
		expect  string
	}{
		{keycloak.FieldRefusal{Field: "email", Code: "error-invalid-email"}, "not valid"},
		{keycloak.FieldRefusal{Field: "firstName", Code: "error-invalid-blank"}, "the first name"},
		{keycloak.FieldRefusal{Field: "username", Code: "error-something-new"}, "the username"},
		{keycloak.FieldRefusal{Code: "error-unknown"}, "this account"},
	} {
		if message := refusalMessage(item.refusal); !strings.Contains(message, item.expect) {
			t.Errorf("%q: expected a message containing %q, got %q", item.refusal.Code, item.expect, message)
		}
	}
}
