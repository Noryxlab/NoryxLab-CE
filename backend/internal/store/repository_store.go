package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/repository"

type RepositoryStore interface {
	ListByUser(userID string) ([]repository.Repository, error)
	GetByID(id string) (repository.Repository, bool, error)
	Create(item repository.Repository) error
	Delete(id string) error
}
