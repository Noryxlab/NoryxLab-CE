package dataset

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Dataset struct {
	ID          string `json:"id"`
	OwnerUserID string `json:"ownerUserId"`
	OwnerType   string `json:"ownerType"`
	OwnerID     string `json:"ownerId"`
	// OwnerName is what a screen shows. An organization is stored by its
	// identifier, which reads as nothing at all on a list.
	OwnerName string `json:"ownerName,omitempty"`
	// AdminVisible marks a row a platform administrator can see only because
	// they administer the platform - they hold no grant on it. Seeing
	// everything is real power over regulated data; the least it can do is say
	// when it is the reason something is on screen.
	AdminVisible   bool   `json:"adminVisible,omitempty"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	Bucket         string `json:"bucket"`
	Prefix         string `json:"prefix"`
	Provider       string `json:"provider"`
	Classification string `json:"classification"`
	Endpoint       string `json:"endpoint,omitempty"`
	Region         string `json:"region,omitempty"`
	AccessRole     string `json:"accessRole,omitempty"`
	// PathLayout is how this dataset's object paths are read: which level
	// holds the subject, the visit, the modality. Absent means the platform
	// falls back to the rule it had compiled in, which is what every dataset
	// declared before this used.
	PathLayout *PathLayout `json:"pathLayout,omitempty"`
	// Card is what this dataset says about itself, in a person's words
	// (ADR-047). Absent means nobody has declared anything, which is a
	// legitimate state and is shown as empty - an unanswered field is
	// information, and filling it with a guess is what that ADR exists to
	// avoid.
	Card *Card `json:"card,omitempty"`
	// Structure is what the last audited pass over this dataset's files found
	// (ADR-047). Absent means nothing has ever opened a file here, which is
	// why a declared pseudonymisation reads as unverified rather than agreeing
	// with a check nobody ran.
	Structure        *StructureScan `json:"structure,omitempty"`
	CredentialName   string         `json:"-"`
	CredentialUserID string         `json:"-"`
	CreatedAt        time.Time      `json:"createdAt"`
	UpdatedAt        time.Time      `json:"updatedAt"`
}

type Access struct {
	DatasetID   string `json:"datasetId"`
	UserID      string `json:"userId,omitempty"`
	SubjectType string `json:"subjectType"`
	SubjectID   string `json:"subjectId"`
	// SubjectName is who that identifier belongs to, resolved for display.
	//
	// The permissions screen showed the raw identifier, so deciding whether
	// the right people had access to a health dataset meant reading four
	// UUIDs and knowing which was which. An access list nobody can read is an
	// access list nobody checks.
	SubjectName string    `json:"subjectName,omitempty"`
	Role        string    `json:"role"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Subject struct {
	Type string
	ID   string
}

func New(ownerUserID, name, description, bucket, prefix, provider, classification, endpoint, region string) Dataset {
	now := time.Now().UTC()
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "" {
		provider = "minio"
	}
	classification = strings.ToLower(strings.TrimSpace(classification))
	if classification != "hds" {
		classification = "non-hds"
	}
	return Dataset{
		ID:               uuid.NewString(),
		OwnerUserID:      strings.TrimSpace(ownerUserID),
		OwnerType:        "user",
		OwnerID:          strings.TrimSpace(ownerUserID),
		CredentialUserID: strings.TrimSpace(ownerUserID),
		Name:             strings.TrimSpace(name),
		Description:      strings.TrimSpace(description),
		Bucket:           strings.TrimSpace(bucket),
		Prefix:           strings.Trim(strings.TrimSpace(prefix), "/"),
		Provider:         provider,
		Classification:   classification,
		Endpoint:         strings.TrimSpace(endpoint),
		Region:           strings.TrimSpace(region),
		CreatedAt:        now,
		UpdatedAt:        now,
	}
}
