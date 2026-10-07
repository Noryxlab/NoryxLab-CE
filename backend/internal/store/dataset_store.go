package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"

type DatasetStore interface {
	ListBySubjects(subjects []dataset.Subject) ([]dataset.Dataset, error)
	ListAll() ([]dataset.Dataset, error)
	GetByID(id string) (dataset.Dataset, bool, error)
	Create(item dataset.Dataset) error
	UpdateMetadata(datasetID, name, description string) error
	// SetPathLayout writes how this dataset's object paths are read - which
	// level holds the subject, the visit, the modality - or clears it so the
	// platform falls back to its compiled rule. A layout stored per dataset is
	// what lets a study that numbers its patients differently be read without
	// anybody writing Go.
	SetPathLayout(datasetID string, layout *dataset.PathLayout) error
	// SetCard writes what this dataset says about itself, in a person's words
	// (ADR-047), or clears it. Beside the layout and for the same reason: a
	// rescan produces a new ontology, so a declared card living there would be
	// lost at every rescan.
	SetCard(datasetID string, card *dataset.Card) error
	// SetStructureScan records what one audited pass over this dataset's files
	// found. Reading inside files is a separate act from listing their keys,
	// and this is where its result lands.
	SetStructureScan(datasetID string, scan *dataset.StructureScan) error
	Delete(id string) error
	ListAccess(datasetID string) ([]dataset.Access, error)
	UpdateOwner(datasetID, ownerType, ownerID string) error
	GetAccess(datasetID, subjectType, subjectID string) (dataset.Access, bool, error)
	SetAccess(item dataset.Access) error
	DeleteAccess(datasetID, subjectType, subjectID string) error
}
