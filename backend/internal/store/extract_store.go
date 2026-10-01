package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"

// ExtractStore persists a named selection of files and the list it froze.
type ExtractStore interface {
	ListByProject(projectID string) ([]extract.Extract, error)
	ListByOntology(ontologyID string) ([]extract.Extract, error)
	GetByID(id string) (extract.Extract, bool, error)
	Create(item extract.Extract, members []extract.Member) error
	ListMembers(extractID string, limit int) ([]extract.Member, error)
	Delete(id string) error
}
