package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/secret"

type SecretStore interface {
	ListByUser(userID string) ([]secret.Secret, error)
	GetByName(userID, name string) (secret.Secret, bool, error)
	Upsert(item secret.Secret) error
	Delete(userID, name string) error
}
