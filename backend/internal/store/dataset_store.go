package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"

type DatasetStore interface {
	ListByUser(userID string) ([]dataset.Dataset, error)
	ListAll() ([]dataset.Dataset, error)
	GetByID(id string) (dataset.Dataset, bool, error)
	Create(item dataset.Dataset) error
	Delete(id string) error
	ListAccess(datasetID string) ([]dataset.Access, error)
	GetAccess(datasetID, userID string) (dataset.Access, bool, error)
	SetAccess(item dataset.Access) error
	DeleteAccess(datasetID, userID string) error
}
