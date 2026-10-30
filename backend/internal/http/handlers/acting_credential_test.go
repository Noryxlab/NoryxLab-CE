package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/apitoken"
)

// The middleware resolves a component's own token into the name the audit
// records, which is what the shared service user could never say.
func TestActingCredentialNamesTheComponent(t *testing.T) {
	handlers, secret := tokenFixture(t, "", func(token *apitoken.Token) {
		token.Component = "platform-validator"
		token.UserID = ""
	})

	var seen auth.Credential
	var found bool
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen, found = auth.CredentialFrom(r.Context())
	})
	request := httptest.NewRequest("DELETE", "/api/v1/datasets/example", nil)
	request.Header.Set("Authorization", "Bearer "+secret)
	handlers.RecordActingCredential(next).ServeHTTP(httptest.NewRecorder(), request)

	if !found {
		t.Fatal("a component token should be resolved at the edge")
	}
	if seen.Component != "platform-validator" || seen.Kind != "component-token" {
		t.Fatalf("unexpected credential %+v", seen)
	}
	if seen.Actor != "service:platform-validator" {
		t.Fatalf("the actor should name the component, got %q", seen.Actor)
	}
	// And the actor the mutation audit will use comes from it, rather than
	// from the service user all components authenticate as.
	if actor := handlers.auditActorUserID(request.WithContext(auth.WithCredential(request.Context(), seen))); actor != "service:platform-validator" {
		t.Fatalf("the audited actor should name the component, got %q", actor)
	}
}

// A person's token stays the person, with the token named beside them.
func TestActingCredentialKeepsAPersonAsTheActor(t *testing.T) {
	handlers, secret := tokenFixture(t, "alice", nil)

	request := httptest.NewRequest("DELETE", "/api/v1/datasets/example", nil)
	request.Header.Set("Authorization", "Bearer "+secret)
	credential, found := handlers.actingCredential(request)
	if !found {
		t.Fatal("a personal token should be resolved at the edge")
	}
	if credential.Actor != "alice" || credential.Component != "" || credential.Kind != "personal-token" {
		t.Fatalf("unexpected credential %+v", credential)
	}
	if credential.TokenID == "" {
		t.Fatal("the token that acted should be named")
	}
}

// A credential that does not verify names nothing: inventing an actor for a
// rejected request would be the opposite of what this records.
func TestAnUnusableTokenNamesNothing(t *testing.T) {
	handlers, _ := tokenFixture(t, "alice", nil)

	request := httptest.NewRequest("DELETE", "/api/v1/datasets/example", nil)
	request.Header.Set("Authorization", "Bearer "+tokenPrefix+"_nope_nope")
	if credential, found := handlers.actingCredential(request); found {
		t.Fatalf("expected no credential, got %+v", credential)
	}
}

// A browser session is left alone, and the actor stays whatever the handler
// resolves for itself.
func TestASessionCarriesNoCredential(t *testing.T) {
	handlers, _ := tokenFixture(t, "alice", nil)

	request := httptest.NewRequest("DELETE", "/api/v1/datasets/example", nil)
	if _, found := handlers.actingCredential(request); found {
		t.Fatal("a request with no platform credential should resolve none")
	}
}
