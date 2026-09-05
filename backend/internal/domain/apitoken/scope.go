package apitoken

import (
	"net/http"
	"sort"
	"strings"
)

// Scopes narrow what a token may do, below what its owner may do.
//
// A token has always acted as its owner, entirely. That is safe in the sense
// that it cannot exceed a person - but a CI job that only needs to launch a
// job holds a credential that can also delete a project, and the first
// security review says so.
//
// The model is deliberately coarse and enforced in one place rather than fine
// and enforced in fifty. A scope names a family of endpoints; the check runs in
// middleware, so a handler added next month is covered without anybody
// remembering to cover it. Fine-grained permissions that leak are worse than
// coarse ones that hold.
type Scope string

const (
	// ScopeRead allows reading anything its owner may read, and changing
	// nothing. This is the scope a dashboard or a report needs.
	ScopeRead Scope = "read"
	// ScopeWorkspaces allows starting and stopping workspaces.
	ScopeWorkspaces Scope = "workspaces"
	// ScopeJobs allows running jobs and builds - the CI case.
	ScopeJobs Scope = "jobs"
	// ScopeFull is what every token had before scopes existed: everything its
	// owner may do. Kept explicit so an unrestricted token is a choice
	// somebody made rather than a default nobody noticed.
	ScopeFull Scope = "full"
)

// AllScopes is what an interface offers, in the order it should offer them:
// least dangerous first.
func AllScopes() []Scope {
	return []Scope{ScopeRead, ScopeWorkspaces, ScopeJobs, ScopeFull}
}

// ValidScope reports whether a string names a scope this platform knows. An
// unknown scope is refused at creation rather than ignored at use: a token
// silently narrower than asked for fails in production, at night.
func ValidScope(value string) bool {
	for _, scope := range AllScopes() {
		if string(scope) == strings.TrimSpace(value) {
			return true
		}
	}
	return false
}

// NormalizeScopes trims, deduplicates and orders a requested set. An empty set
// means ScopeFull: tokens created before scopes existed keep behaving exactly
// as they did, which is the only migration that cannot break a running
// pipeline.
func NormalizeScopes(requested []string) []string {
	seen := map[string]struct{}{}
	for _, value := range requested {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		seen[value] = struct{}{}
	}
	if len(seen) == 0 {
		return []string{string(ScopeFull)}
	}
	scopes := make([]string, 0, len(seen))
	for value := range seen {
		scopes = append(scopes, value)
	}
	sort.Strings(scopes)
	return scopes
}

// Permits reports whether a token holding these scopes may make this request.
//
// Reading is allowed by every scope: a credential that may launch a job it
// cannot then read the result of would be useless, and would push people back
// to an unrestricted token - which is the outcome this is trying to avoid.
func Permits(scopes []string, method, path string) bool {
	if len(scopes) == 0 {
		return true
	}
	for _, scope := range scopes {
		if Scope(scope) == ScopeFull {
			return true
		}
	}
	if method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions {
		return true
	}

	for _, scope := range scopes {
		switch Scope(scope) {
		case ScopeWorkspaces:
			if underAny(path, "/api/v1/workspaces") {
				return true
			}
		case ScopeJobs:
			if underAny(path, "/api/v1/jobs", "/api/v1/builds", "/api/v1/cronjobs") {
				return true
			}
		}
	}
	return false
}

// Explain says which scope a refused request would have needed, so the error a
// pipeline receives is actionable rather than "forbidden".
func Explain(method, path string) string {
	switch {
	case underAny(path, "/api/v1/workspaces"):
		return string(ScopeWorkspaces)
	case underAny(path, "/api/v1/jobs", "/api/v1/builds", "/api/v1/cronjobs"):
		return string(ScopeJobs)
	case method == http.MethodGet || method == http.MethodHead:
		return string(ScopeRead)
	default:
		return string(ScopeFull)
	}
}

func underAny(path string, prefixes ...string) bool {
	for _, prefix := range prefixes {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
