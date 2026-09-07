package handlers

import (
	"net/http/httptest"
	"strings"
	"testing"
)

// Adding a repository with a token failed: the form sent "secret", meaning "a
// token kept in a secret", and the API answered "authType must be persat or
// prat" - words nobody had been shown, about a distinction the form did not
// offer.
func TestARepositoryTokenIsAcceptedByAnyReasonableName(t *testing.T) {
	for _, given := range []string{"", "secret", "token", "pat", "PERSAT"} {
		got, ok := normalizeRepositoryAuthType(httptest.NewRecorder(), given, "my-token")
		if !ok || got != "persat" {
			t.Errorf("%q should mean a personal token, got %q (ok=%v)", given, got, ok)
		}
	}
	for _, given := range []string{"prat", "repository", "deploy"} {
		got, ok := normalizeRepositoryAuthType(httptest.NewRecorder(), given, "my-token")
		if !ok || got != "prat" {
			t.Errorf("%q should mean a repository token, got %q (ok=%v)", given, got, ok)
		}
	}
}

func TestARepositoryWithNoSecretNeedsNoTokenKind(t *testing.T) {
	got, ok := normalizeRepositoryAuthType(httptest.NewRecorder(), "anything", "")
	if !ok || got != "none" {
		t.Errorf("a public repository needs no token, got %q (ok=%v)", got, ok)
	}
}

func TestAnUnknownTokenKindSaysWhatIsAccepted(t *testing.T) {
	recorder := httptest.NewRecorder()
	if _, ok := normalizeRepositoryAuthType(recorder, "kerberos", "my-token"); ok {
		t.Fatal("an unknown kind should be refused")
	}
	if body := recorder.Body.String(); !strings.Contains(body, "personal token") || !strings.Contains(body, "repository token") {
		t.Errorf("the refusal should name what is accepted, in words: %s", body)
	}
}
