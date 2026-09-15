package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/repository"
)

func TestValidateRepositoryGitIdentity(t *testing.T) {
	tests := []struct {
		name      string
		author    string
		email     string
		expectErr bool
	}{
		{name: "empty fallback identity"},
		{name: "complete identity", author: "Git Author", email: "git@example.org"},
		{name: "missing email", author: "Git Author", expectErr: true},
		{name: "missing name", email: "git@example.org", expectErr: true},
		{name: "invalid email", author: "Git Author", email: "not-an-email", expectErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateRepositoryGitIdentity(test.author, test.email)
			if (err != nil) != test.expectErr {
				t.Fatalf("error=%v expectErr=%v", err, test.expectErr)
			}
		})
	}
}

func TestSetRepositoryValidation(t *testing.T) {
	item := repository.Repository{}
	setRepositoryValidation(&item, nil)
	if !item.Reachable || item.ValidationError != "" || item.LastValidatedAt == nil {
		t.Fatalf("unexpected successful validation state: %#v", item)
	}

	setRepositoryValidation(&item, errors.New("authentication failed"))
	if item.Reachable || item.ValidationError != "authentication failed" || item.LastValidatedAt == nil {
		t.Fatalf("unexpected failed validation state: %#v", item)
	}
}

// A provider that wants a credential does not always say 401.
//
// Azure DevOps redirects to an Entra sign-in page. Following that redirect
// landed on an HTML login form answering 203, which reached the user as
// "unexpected status=203" - true, useless, and not the actual problem. The
// worse version is a sign-in page answering 200: the platform would then have
// reported a repository it cannot read as validated.
func TestASignInRedirectIsAnAuthenticationFailureNotAnOddity(t *testing.T) {
	for _, status := range []int{http.StatusFound, http.StatusMovedPermanently, http.StatusNonAuthoritativeInfo} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Location", "https://login.example.com/signin")
			w.WriteHeader(status)
		}))

		_, err := checkRepository(server.URL+"/org/repo", "")
		server.Close()

		if err == nil {
			t.Fatalf("status %d validated a repository behind a sign-in page", status)
		}
		if !strings.Contains(err.Error(), "authentication required") {
			t.Errorf("status %d reported %q, which does not name the real problem", status, err)
		}
		// The message has to carry the fix, because the credential format is
		// the thing nobody guesses.
		if !strings.Contains(err.Error(), "personal-access-token") {
			t.Errorf("status %d does not say how to authenticate: %q", status, err)
		}
	}
}
