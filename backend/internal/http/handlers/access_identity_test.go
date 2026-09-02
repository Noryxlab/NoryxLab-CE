package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Authentication regression tests.
//
// The header identity was honoured unconditionally: any caller that could
// reach this service could act as any user by naming them, with no token and
// no session. On the deployed platform, `X-Noryx-User: stef` returned that
// user's real projects from an unauthenticated pod. The bearer check was added
// in front of it and this path was never closed behind.

func request(header, value string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	if header != "" {
		r.Header.Set(header, value)
	}
	return r
}

func TestUserHeaderIsRefusedOutsideHeaderAuthMode(t *testing.T) {
	handlers := Handlers{authMode: "oidc"}
	recorder := httptest.NewRecorder()

	if _, ok := handlers.requireIdentity(recorder, request(userHeader, "stef")); ok {
		t.Fatal("a bare user header authenticated in oidc mode")
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestUserHeaderStillWorksInHeaderAuthMode(t *testing.T) {
	// The development mode has to keep working, or every local setup breaks.
	handlers := Handlers{authMode: "header"}
	identity, ok := handlers.requireIdentity(httptest.NewRecorder(), request(userHeader, "stef"))
	if !ok {
		t.Fatal("header mode refused a named user")
	}
	if identity.UserID() != "stef" {
		t.Fatalf("identity = %q, want stef", identity.UserID())
	}
}

func TestServiceTokenAuthenticatesAPlatformComponent(t *testing.T) {
	handlers := Handlers{authMode: "oidc", serviceToken: "s3cr3t"}
	req := request(serviceHeader, "s3cr3t")
	req.Header.Set(userHeader, "platform-validator")

	identity, ok := handlers.requireIdentity(httptest.NewRecorder(), req)
	if !ok {
		t.Fatal("a valid service token was refused")
	}
	// Named so a backup run records which component asked, rather than
	// attributing every automated action to one opaque identity.
	if identity.UserID() != "platform-validator" {
		t.Fatalf("identity = %q, want platform-validator", identity.UserID())
	}
	if !identity.HasRole(globalAdminRole) {
		t.Fatal("a platform component cannot trigger a backup without admin rights")
	}
}

func TestAWrongOrAbsentServiceTokenAuthenticatesNothing(t *testing.T) {
	handlers := Handlers{authMode: "oidc", serviceToken: "s3cr3t"}
	for name, req := range map[string]*http.Request{
		"wrong token":  request(serviceHeader, "not-it"),
		"empty header": request(serviceHeader, ""),
		"no header":    request("", ""),
	} {
		recorder := httptest.NewRecorder()
		if _, ok := handlers.requireIdentity(recorder, req); ok {
			t.Fatalf("%s authenticated", name)
		}
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status = %d, want 401", name, recorder.Code)
		}
	}
}

// A deployment that forgets the secret must refuse its own services, never
// accept an empty header from anyone.
func TestAnUnconfiguredServiceTokenMatchesNothing(t *testing.T) {
	handlers := Handlers{authMode: "oidc", serviceToken: ""}
	recorder := httptest.NewRecorder()
	if _, ok := handlers.requireIdentity(recorder, request(serviceHeader, "")); ok {
		t.Fatal("an empty configured token accepted an empty presented token")
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}
