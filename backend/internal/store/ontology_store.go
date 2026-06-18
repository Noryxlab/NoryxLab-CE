package store

import "encoding/json"

type ProjectOntologyStore interface {
	GetProjectOntology(projectID string) (json.RawMessage, bool, error)
	UpsertProjectOntology(projectID, datasetID string, manifest json.RawMessage, generatedBy string) error
}
