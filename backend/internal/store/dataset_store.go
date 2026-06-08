package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"

type DatasetStore interface {
	ListBySubjects(subjects []dataset.Subject) ([]dataset.Dataset, error)
	ListAll() ([]dataset.Dataset, error)
	GetByID(id string) (dataset.Dataset, bool, error)
	Create(item dataset.Dataset) error
	Delete(id string) error
	ListAccess(datasetID string) ([]dataset.Access, error)
	UpdateOwner(datasetID, ownerType, ownerID string) error
	GetAccess(datasetID, subjectType, subjectID string) (dataset.Access, bool, error)
	SetAccess(item dataset.Access) error
	DeleteAccess(datasetID, subjectType, subjectID string) error
}
