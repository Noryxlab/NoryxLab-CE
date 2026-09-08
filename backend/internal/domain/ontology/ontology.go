package ontology

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Ontology struct {
	ID          string `json:"id"`
	OwnerUserID string `json:"ownerUserId"`
	OwnerType   string `json:"ownerType"`
	OwnerID     string `json:"ownerId"`
	// OwnerName is what a screen shows, resolved from the directory for an
	// organization and equal to the username for a person.
	OwnerName string `json:"ownerName,omitempty"`
	// AdminVisible marks a row a platform administrator can see only because
	// they administer the platform - they hold no grant on it. Seeing
	// everything is real power over regulated data; the least it can do is say
	// when it is the reason something is on screen.
	AdminVisible     bool            `json:"adminVisible,omitempty"`
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
		Status:           "active",
		Manifest:         append(json.RawMessage(nil), manifest...),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}

// Object is one file the scan recognised, kept with the subject, visit and
// modality it was filed under.
//
// The manifest keeps three sample paths per modality, which is enough to show
// someone what the data looks like and not nearly enough to build a cohort
// from: a cohort is a list of files, it has to be frozen at the moment it is
// declared, and it has to still name the same files when someone reproduces
// the study a year later. So the paths are kept.
//
// Only the key and its size are stored - never the contents. The platform's
// business with a regulated bucket is to say what is in it.
type Object struct {
	OntologyID string `json:"ontologyId"`
	Path       string `json:"path"`
	SubjectID  string `json:"subjectId"`
	Visit      string `json:"visit"`
	Modality   string `json:"modality"`
	SizeBytes  int64  `json:"sizeBytes"`
}

// ObjectFilter selects the files a cohort is made of. An empty list means "no
// constraint on this axis", never "nothing": a cohort defined by modality
// alone must span every subject that carries it.
type ObjectFilter struct {
	Subjects   []string
	Modalities []string
	Visits     []string
	Limit      int
}
