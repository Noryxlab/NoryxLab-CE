package storageendpoint

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Endpoint struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Provider       string    `json:"provider"`
	EndpointURL    string    `json:"endpoint"`
	Region         string    `json:"region"`
	Classification string    `json:"classification"`
	Purpose        string    `json:"purpose"`
	UseSSL         bool      `json:"useSSL"`
	DefaultBackup  bool      `json:"defaultBackup"`
	DefaultDataset bool      `json:"defaultDataset"`
	SecretName     string    `json:"-"`
	Status         string    `json:"status"`
	StatusMessage  string    `json:"statusMessage,omitempty"`
	LastCheckedAt  time.Time `json:"lastCheckedAt,omitempty"`
	CreatedBy      string    `json:"createdBy"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func New(createdBy, name, provider, endpointURL, region, classification, purpose string, useSSL bool) Endpoint {
	now := time.Now().UTC()
	id := uuid.NewString()
	provider = normalize(provider, "s3")
	classification = normalizeClassification(classification)
	purpose = normalizePurpose(purpose)
	return Endpoint{
		ID:             id,
		Name:           strings.TrimSpace(name),
		Provider:       provider,
		EndpointURL:    strings.TrimSpace(endpointURL),
		Region:         normalize(region, "us-east-1"),
		Classification: classification,
		Purpose:        purpose,
		UseSSL:         useSSL,
		SecretName:     "noryx-storage-endpoint-" + id,
		Status:         "unchecked",
		CreatedBy:      strings.TrimSpace(createdBy),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

func normalize(value, fallback string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return fallback
	}
	return value
}

func normalizeClassification(value string) string {
	switch normalize(value, "internal") {
	case "hds", "non-hds", "internal", "backup":
		return normalize(value, "internal")
	default:
		return "internal"
	}
}

func normalizePurpose(value string) string {
	switch normalize(value, "general") {
	case "backup", "dataset", "artifact", "staging", "general":
		return normalize(value, "general")
	default:
		return "general"
	}
}
