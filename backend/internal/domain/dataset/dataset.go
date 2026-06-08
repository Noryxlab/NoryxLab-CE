package dataset

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Dataset struct {
	ID             string    `json:"id"`
	OwnerUserID    string    `json:"ownerUserId"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	Bucket         string    `json:"bucket"`
	Prefix         string    `json:"prefix"`
	Provider       string    `json:"provider"`
	Classification string    `json:"classification"`
	Endpoint       string    `json:"endpoint,omitempty"`
	Region         string    `json:"region,omitempty"`
	AccessRole     string    `json:"accessRole,omitempty"`
	CredentialName string    `json:"-"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

type Access struct {
	DatasetID string    `json:"datasetId"`
	UserID    string    `json:"userId"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
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
		ID:             uuid.NewString(),
		OwnerUserID:    strings.TrimSpace(ownerUserID),
		Name:           strings.TrimSpace(name),
		Description:    strings.TrimSpace(description),
		Bucket:         strings.TrimSpace(bucket),
		Prefix:         strings.Trim(strings.TrimSpace(prefix), "/"),
		Provider:       provider,
		Classification: classification,
		Endpoint:       strings.TrimSpace(endpoint),
		Region:         strings.TrimSpace(region),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
