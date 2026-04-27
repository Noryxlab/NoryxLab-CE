package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"

type DatasetStore interface {
	ListByUser(userID string) ([]dataset.Dataset, error)
	GetByID(id string) (dataset.Dataset, bool, error)
	Create(item dataset.Dataset) error
}
