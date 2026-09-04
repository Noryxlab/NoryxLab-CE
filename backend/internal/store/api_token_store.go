package store

import (
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/apitoken"
)

// APITokenStore keeps the credentials users present instead of a session.
//
// Lookup is by identifier rather than by scanning every token, so
// authentication does not get slower as more are issued.
type APITokenStore interface {
	Put(token apitoken.Token) error
	Get(id string) (apitoken.Token, bool, error)
	ListByUser(userID string) ([]apitoken.Token, error)
	// Revoke stamps rather than deletes: a token that vanishes leaves its
	// owner wondering whether it ever existed, and an auditor unable to say
	// when access ended.
	Revoke(id, userID string, at time.Time) (bool, error)
	Touch(id string, at time.Time) error
}
