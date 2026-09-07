package handlers

import (
	"net/http"
	"sort"
	"strings"
)

// What a token is allowed to do, beside cloning.
//
// Cloning a repository needs `repo`, and nothing else. The token this was
// written for carried admin:enterprise, admin:org, delete_repo,
// write:packages, workflow and fifteen others - it could delete every
// repository in the organisation, and it was about to be injected as an
// environment variable into every workspace its owner launches, where any
// notebook can print it with `env`.
//
// GitHub says what a token can do in a response header. The platform is
// already making that call to check the repository is reachable, so it costs
// nothing to look, and a platform that knows should say so.
//
// This is a warning, never a refusal: the scopes belong to whoever created the
// token, some organisations really do issue one broad token, and a platform
// that refused would be telling people how to run their GitHub account.

// scopesForCloning is what a clone actually uses. Everything else is excess.
var scopesForCloning = map[string]bool{
	"repo":            true,
	"public_repo":     true,
	"repo:status":     true,
	"read:packages":   true,
	"read:org":        true,
	"read:user":       true,
	"user:email":      true,
	"repo_deployment": true,
}

// alarmingScopes are the ones worth naming first in a warning: they destroy or
// administer rather than read.
var alarmingScopes = map[string]bool{
	"delete_repo":                  true,
	"admin:org":                    true,
	"admin:enterprise":             true,
	"admin:public_key":             true,
	"admin:ssh_signing_key":        true,
	"admin:org_hook":               true,
	"admin:repo_hook":              true,
	"admin:gpg_key":                true,
	"delete:packages":              true,
	"write:packages":               true,
	"write:org":                    true,
	"write:network_configurations": true,
	"workflow":                     true,
	"user":                         true,
	"gist":                         true,
	"codespace":                    true,
	"project":                      true,
	"audit_log":                    true,
}

// tokenScopesFromResponse reads the scopes GitHub reports for the token that
// made the request. An empty answer means the provider does not say - a
// fine-grained GitHub token reports nothing here, and neither does GitLab -
// and silence is not evidence of anything.
func tokenScopesFromResponse(resp *http.Response) []string {
	if resp == nil {
		return nil
	}
	raw := strings.TrimSpace(resp.Header.Get("X-OAuth-Scopes"))
	if raw == "" {
		return nil
	}
	out := []string{}
	for _, scope := range strings.Split(raw, ",") {
		if trimmed := strings.TrimSpace(scope); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	sort.Strings(out)
	return out
}

// excessiveTokenScopes answers with what this token can do beyond cloning,
// most alarming first, so a warning that has to be short says the frightening
// part.
func excessiveTokenScopes(scopes []string) []string {
	excess := []string{}
	for _, scope := range scopes {
		if !scopesForCloning[strings.ToLower(scope)] {
			excess = append(excess, scope)
		}
	}
	sort.SliceStable(excess, func(i, j int) bool {
		left, right := alarmingScopes[strings.ToLower(excess[i])], alarmingScopes[strings.ToLower(excess[j])]
		if left != right {
			return left
		}
		return excess[i] < excess[j]
	})
	return excess
}

// tokenCanDestroy says whether the excess includes something that deletes or
// administers, which is the difference between "broader than needed" and "this
// can delete your repositories".
func tokenCanDestroy(scopes []string) bool {
	for _, scope := range scopes {
		if alarmingScopes[strings.ToLower(scope)] {
			return true
		}
	}
	return false
}
