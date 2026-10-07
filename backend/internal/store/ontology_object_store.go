package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"

type OntologyStore interface {
	ListBySubjects(subjects []ontology.Subject) ([]ontology.Ontology, error)
	ListAll() ([]ontology.Ontology, error)
	GetByID(id string) (ontology.Ontology, bool, error)
	Create(item ontology.Ontology) error
	UpdateMetadata(ontologyID, name, description string) error

	// ReplaceManifest is a re-scan of the same source.
	//
	// Scanning always created a new row, so scanning a dataset twice left two
	// ontologies with the same name over the same bucket and no way to tell
	// them apart - which is how EMSE ended up with two PREMYOM1000. A re-scan
	// is a new photograph of the same thing, so it replaces the picture and
	// keeps the object: its identifier, its name, its owner and the extracts
	// that point at it.
	ReplaceManifest(ontologyID string, manifest []byte, generatedBy string) error

	// SetCard writes what this ontology says the data means, in a person's
	// words (ADR-047), or clears it. Beside the manifest rather than inside
	// it: a rescan replaces the photograph and must not touch the declaration.
	SetCard(ontologyID string, card *ontology.Card) error
	// SetStructureScan records what one audited pass over the files found.
	// Reading inside files is a separate act from listing their keys, and this
	// is where its result lands.
	SetStructureScan(ontologyID string, scan *ontology.StructureScan) error
	Delete(id string) error
	ListAccess(ontologyID string) ([]ontology.Access, error)
	UpdateOwner(ontologyID, ownerType, ownerID string) error
	GetAccess(ontologyID, subjectType, subjectID string) (ontology.Access, bool, error)
	SetAccess(item ontology.Access) error
	DeleteAccess(ontologyID, subjectType, subjectID string) error

	// The paths a scan recognised. An extract is a list of files, so it needs
	// them; the manifest's three samples per modality never could be one.
	ReplaceObjects(ontologyID string, objects []ontology.Object) error
	ListObjects(ontologyID string, filter ontology.ObjectFilter) ([]ontology.Object, error)
	CountObjects(ontologyID string) (int, error)
}
