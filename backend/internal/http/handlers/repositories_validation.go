package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// validateRepositoryConnectivity checks the repository answers, and reports
// what the token is allowed to do while it is there: the call is made anyway,
// and the provider says so in a header.
func validateRepositoryConnectivity(repoURL, secretValue string) error {
	_, err := checkRepository(repoURL, secretValue)
	return err
}

func checkRepository(repoURL, secretValue string) ([]string, error) {
	host, repoPath, err := parseRepoLocation(repoURL)
	if err != nil {
		return nil, err
	}

	// Redirects are not followed while probing.
	//
	// A provider that wants a credential and did not get one does not always
	// say 401. Azure DevOps redirects to an Entra sign-in page, and following
	// that lands on an HTML login form - which answered 203 here and came back
	// to the user as "unexpected status=203", a message nobody can act on. The
	// worse version of the same thing is a sign-in page answering 200, which
	// would have validated a repository the platform cannot read.
	//
	// So the first response is the answer, and a redirect away from the
	// repository is read as what it is.
	client := &http.Client{
		Timeout: 12 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	reqURL := ""
	switch host {
	case "github.com":
		reqURL = "https://api.github.com/repos/" + repoPath
	case "gitlab.com":
		reqURL = "https://gitlab.com/api/v4/projects/" + url.PathEscape(repoPath)
	default:
		// Generic smart-HTTP git endpoint probe.
		base := strings.TrimSuffix(strings.TrimSpace(repoURL), ".git")
		reqURL = strings.TrimSuffix(base, "/") + ".git/info/refs?service=git-upload-pack"
	}

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid validation request")
	}
	req.Header.Set("User-Agent", "noryx-ce-repo-validator")
	applyRepositoryAuthHeaders(req, host, secretValue)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unreachable")
	}
	_ = resp.Body.Close()
	scopes := tokenScopesFromResponse(resp)

	switch resp.StatusCode {
	case http.StatusOK:
		return scopes, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return scopes, fmt.Errorf("authentication failed (status=%d)", resp.StatusCode)
	case http.StatusNotFound:
		// A private repository answers 404 to a reader it does not recognise:
		// the server hides existence rather than admitting to it, so "absent"
		// and "not allowed" are the same answer and the message must not pick
		// one. Saying "repository not found" to somebody looking at the
		// repository in another tab is how a correct probe becomes an
		// unhelpful one.
		return scopes, fmt.Errorf("the address answered 404: either it does not exist, " +
			"or the token cannot see it - a private repository gives the same answer to both. " +
			"Check the address, and that the token carries read access to the code")
	case http.StatusNonAuthoritativeInfo:
		// Azure DevOps answers this with a sign-in page rather than 401.
		return scopes, fmt.Errorf("authentication required: the provider asked for a sign-in. For Azure DevOps, store the secret as user:personal-access-token")
	default:
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			return scopes, fmt.Errorf("authentication required: the provider redirected to a sign-in page. For Azure DevOps, store the secret as user:personal-access-token")
		}
		// The probed address, not only the status: this probe is not the
		// repository's own URL - it is built from it - and a caller comparing
		// a surprising status against the address they typed is comparing two
		// different things.
		return scopes, fmt.Errorf("unexpected status=%d from %s", resp.StatusCode, reqURL)
	}
}

// splitRepositoryCredential reads the stored secret the one way the whole
// product reads it: "user:token", or a bare token belonging to "oauth2".
//
// The same rule the clone applies in the workspace (repositoryBootstrapLines).
// Written down once here because the last time the two paths each had their
// own idea of what a secret meant, validation authenticated correctly and the
// clone sent the whole string as a password - so a repository reported
// reachable would not clone.
func splitRepositoryCredential(secretValue string) (user, pass string) {
	secretValue = strings.TrimSpace(secretValue)
	if before, after, found := strings.Cut(secretValue, ":"); found {
		return before, after
	}
	return "oauth2", secretValue
}

// applyRepositoryAuthHeaders authenticates the probe the way the host expects.
//
// The distinction that matters: github.com and gitlab.com are probed through
// their *API*, where a bare token legitimately is a bearer token. Every other
// host is probed through git's own smart-HTTP endpoint, which speaks Basic and
// nothing else - Azure DevOps rejects a personal access token presented as a
// bearer, and answers with a sign-in redirect that reads to the caller as an
// unreachable repository.
//
// Sending Bearer there was therefore guaranteed to fail for the most natural
// way to store a token, which is to paste the token and nothing else.
func applyRepositoryAuthHeaders(req *http.Request, host, secretValue string) {
	secretValue = strings.TrimSpace(secretValue)
	if secretValue == "" {
		return
	}
	user, pass := splitRepositoryCredential(secretValue)

	switch host {
	case "github.com", "gitlab.com":
		if creds := strings.SplitN(secretValue, ":", 2); len(creds) == 2 && creds[0] != "" {
			req.SetBasicAuth(creds[0], creds[1])
		} else {
			req.Header.Set("Authorization", "Bearer "+secretValue)
		}
		if host == "gitlab.com" {
			req.Header.Set("PRIVATE-TOKEN", secretValue)
		}
	default:
		req.SetBasicAuth(user, pass)
	}
}
