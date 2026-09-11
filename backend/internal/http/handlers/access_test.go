package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
)

// A shared-secret caller is a service, whatever name it puts on its request.
//
// The name used to be written into the identity's username. Everything that
// resolves a user then treated the service as that person - their
// organizations, their project roles, their name in the audit trail - while
// the identity still carried the global administrator role. A component could
// read every dataset on the platform and have it recorded as the work of a
// named researcher who had done nothing. Over-permission is a defect;
// recording it under somebody else's name is a worse one.
func TestServiceTokenNeverBecomesTheNamedUser(t *testing.T) {
	h := Handlers{serviceToken: "s3cret"}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/datasets", nil)
	request.Header.Set("X-Noryx-Service-Token", "s3cret")
	request.Header.Set("X-Noryx-User", "barantok")

	identity, ok := h.serviceIdentity(request)
	if !ok {
		t.Fatal("a valid service token must authenticate")
	}
	if identity.UserID() == "barantok" {
		t.Fatal("the service adopted the name it was given: every user lookup downstream now resolves that person")
	}
	if identity.UserID() != auth.ServiceUsername {
		t.Fatalf("service identity = %q, want %q", identity.UserID(), auth.ServiceUsername)
	}
	if identity.DeclaredBy != "barantok" {
		t.Fatalf("the declared name must survive for the audit trail, got %q", identity.DeclaredBy)
	}
	if !identity.IsService() {
		t.Fatal("a shared-secret caller must be recognisable as a service")
	}
}

// An unnamed service is still a service, and still not a person.
func TestServiceTokenWithoutANameIsStillAService(t *testing.T) {
	h := Handlers{serviceToken: "s3cret"}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/datasets", nil)
	request.Header.Set("X-Noryx-Service-Token", "s3cret")

	identity, ok := h.serviceIdentity(request)
	if !ok || !identity.IsService() || identity.UserID() != auth.ServiceUsername {
		t.Fatalf("identity = %+v, want the service identity", identity)
	}
}

// An issued credential is presented whole, or it authenticates nowhere.
//
// A token is "<prefix>_<id>_<secret>" and nothing else parses. Handing back
// only the secret half produced a credential that failed on every call with
// "invalid bearer token" - which reads like a revoked token and is a
// malformed one.
func TestAnIssuedTokenIsReturnedInThePresentedForm(t *testing.T) {
	id, secret, err := newTokenParts()
	if err != nil {
		t.Fatal(err)
	}
	presented := tokenPrefix + "_" + id + "_" + secret

	gotID, gotSecret, ok := parseToken(presented)
	if !ok {
		t.Fatalf("the form an endpoint returns must be the form the parser accepts: %q", presented)
	}
	if gotID != id || gotSecret != secret {
		t.Fatalf("round trip changed the token: %q/%q", gotID, gotSecret)
	}
	if _, _, ok := parseToken(secret); ok {
		t.Fatal("the bare secret must not parse: returning it hands out a credential that works nowhere")
	}
}
