package auth

import "strings"

type Identity struct {
	Subject  string
	Username string
	Email    string
	Roles    map[string]struct{}
	// Scopes is set when the caller authenticated with a restricted API token.
	// Empty means unrestricted: a browser session, an OIDC bearer, or a token
	// created before scopes existed.
	Scopes []string
}

func (i Identity) UserID() string {
	if i.Username != "" {
		return i.Username
	}
	if i.Email != "" {
		return i.Email
	}
	return i.Subject
}

func (i Identity) HasRole(role string) bool {
	_, ok := i.Roles[role]
	return ok
}

func (i Identity) MatchesUsername(username string) bool {
	return strings.EqualFold(strings.TrimSpace(i.Username), strings.TrimSpace(username))
}

func (i Identity) MatchesEmail(email string) bool {
	return strings.EqualFold(strings.TrimSpace(i.Email), strings.TrimSpace(email))
}

type Verifier interface {
	VerifyBearerToken(token string) (Identity, error)
}
