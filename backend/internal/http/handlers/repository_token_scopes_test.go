package handlers

import (
	"net/http"
	"strings"
	"testing"
)

// The token this was written for: a classic GitHub PAT with no expiry and
// twenty-one scopes, of which cloning uses one.
const observedScopes = "admin:enterprise, admin:gpg_key, admin:org, admin:org_hook, admin:public_key, " +
	"admin:repo_hook, admin:ssh_signing_key, audit_log, codespace, copilot, delete:packages, " +
	"delete_repo, gist, notifications, project, repo, user, workflow, write:discussion, " +
	"write:network_configurations, write:packages"

func responseWithScopes(raw string) *http.Response {
	header := http.Header{}
	if raw != "" {
		header.Set("X-OAuth-Scopes", raw)
	}
	return &http.Response{Header: header}
}

func TestTheExcessOfARealTokenIsReported(t *testing.T) {
	scopes := tokenScopesFromResponse(responseWithScopes(observedScopes))
	if len(scopes) != 21 {
		t.Fatalf("expected 21 scopes, got %d", len(scopes))
	}

	excess := excessiveTokenScopes(scopes)
	for _, needed := range []string{"repo"} {
		for _, got := range excess {
			if got == needed {
				t.Errorf("%s is what cloning uses and must not be reported as excess", needed)
			}
		}
	}
	if !tokenCanDestroy(excess) {
		t.Error("a token with delete_repo and admin:org should be reported as destructive")
	}
	// The frightening ones first: a warning that has to be short says the
	// frightening part.
	if !strings.HasPrefix(strings.Join(excess[:3], ","), "admin:enterprise") {
		t.Errorf("the alarming scopes should lead: %v", excess[:5])
	}
}

func TestATokenScopedForCloningRaisesNothing(t *testing.T) {
	scopes := tokenScopesFromResponse(responseWithScopes("repo, read:org"))
	if excess := excessiveTokenScopes(scopes); len(excess) != 0 {
		t.Errorf("a token scoped for cloning should raise nothing, got %v", excess)
	}
	if tokenCanDestroy(nil) {
		t.Error("nothing is not destructive")
	}
}

// A fine-grained GitHub token reports no scopes at all, and GitLab reports
// none either. Silence is not evidence of a narrow token, so it raises
// nothing rather than a false all-clear.
func TestAProviderThatSaysNothingRaisesNothing(t *testing.T) {
	if scopes := tokenScopesFromResponse(responseWithScopes("")); len(scopes) != 0 {
		t.Errorf("expected no scopes, got %v", scopes)
	}
	if scopes := tokenScopesFromResponse(nil); scopes != nil {
		t.Errorf("a missing response says nothing, got %v", scopes)
	}
}
