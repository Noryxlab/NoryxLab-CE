package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/secret"

type SecretStore interface {
	ListByUser(userID string) ([]secret.Secret, error)
	// ListAll returns every secret across users. Reserved for platform-wide
	// operations such as backup; never expose it on a user-facing endpoint.
	ListAll() ([]secret.Secret, error)
	GetByName(userID, name string) (secret.Secret, bool, error)
	Upsert(item secret.Secret) error
	Delete(userID, name string) error
}
