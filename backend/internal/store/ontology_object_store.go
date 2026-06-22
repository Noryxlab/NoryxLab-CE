package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"

type OntologyStore interface {
	ListBySubjects(subjects []ontology.Subject) ([]ontology.Ontology, error)
	ListAll() ([]ontology.Ontology, error)
	GetByID(id string) (ontology.Ontology, bool, error)
	Create(item ontology.Ontology) error
	UpdateMetadata(ontologyID, name, description string) error
	Delete(id string) error
	ListAccess(ontologyID string) ([]ontology.Access, error)
	UpdateOwner(ontologyID, ownerType, ownerID string) error
	GetAccess(ontologyID, subjectType, subjectID string) (ontology.Access, bool, error)
	SetAccess(item ontology.Access) error
	DeleteAccess(ontologyID, subjectType, subjectID string) error
}
