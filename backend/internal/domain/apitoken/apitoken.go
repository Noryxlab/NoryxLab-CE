// Package apitoken holds the credentials a user presents instead of a browser
// session.
//
// A researcher calling the API from a CI job or a notebook has, until now, had
// no honest option: the platform authenticates people through Keycloak, and a
// pipeline has no browser. The alternatives people reach for otherwise are
// worse - sharing a password, or copying a short-lived bearer token out of the
// browser and wondering why it stops working an hour later.
//
// Distinct from the platform service token, which identifies a component of the
// platform itself and carries administrator rights. A token here acts as one
// person and can never exceed what that person may do.
package apitoken

import "time"

// Token is the stored half of a credential. The secret is never here: only its
// hash, so a copy of the database is not a set of working credentials.
type Token struct {
	ID     string `json:"id"`
	UserID string `json:"userId"`
	// Name is what the owner called it, so revoking the right one does not
	// require guessing. "gitlab-ci" beats "token 3".
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`

	ExpiresAt  *time.Time `json:"expiresAt,omitempty"`
	RevokedAt  *time.Time `json:"revokedAt,omitempty"`
	LastUsedAt *time.Time `json:"lastUsedAt,omitempty"`

	SecretHash []byte `json:"-"`
}

// Active reports whether the token may be used now.
func (t Token) Active(now time.Time) bool {
	if t.RevokedAt != nil {
		return false
	}
	if t.ExpiresAt != nil && !now.Before(*t.ExpiresAt) {
		return false
	}
	return true
}
