package ontology

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Ontology struct {
	ID               string          `json:"id"`
	OwnerUserID      string          `json:"ownerUserId"`
	OwnerType        string          `json:"ownerType"`
	OwnerID          string          `json:"ownerId"`
	Name             string          `json:"name"`
	Description      string          `json:"description"`
	SourceType       string          `json:"sourceType"`
	SourceID         string          `json:"sourceId"`
	SourceName       string          `json:"sourceName"`
	InferenceProfile string          `json:"inferenceProfile"`
	Status           string          `json:"status"`
	Manifest         json.RawMessage `json:"manifest"`
	AccessRole       string          `json:"accessRole,omitempty"`
	CreatedAt        time.Time       `json:"createdAt"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

type Access struct {
	OntologyID  string    `json:"ontologyId"`
	UserID      string    `json:"userId,omitempty"`
	SubjectType string    `json:"subjectType"`
	SubjectID   string    `json:"subjectId"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Subject struct {
	Type string
	ID   string
}

func New(ownerUserID, name, description, sourceType, sourceID, sourceName, inferenceProfile string, manifest json.RawMessage) Ontology {
	now := time.Now().UTC()
	return Ontology{
		ID:               uuid.NewString(),
		OwnerUserID:      strings.TrimSpace(ownerUserID),
		OwnerType:        "user",
		OwnerID:          strings.TrimSpace(ownerUserID),
		Name:             strings.TrimSpace(name),
		Description:      strings.TrimSpace(description),
		SourceType:       strings.ToLower(strings.TrimSpace(sourceType)),
		SourceID:         strings.TrimSpace(sourceID),
		SourceName:       strings.TrimSpace(sourceName),
		InferenceProfile: strings.TrimSpace(inferenceProfile),
		Status:           "draft",
		Manifest:         append(json.RawMessage(nil), manifest...),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}
