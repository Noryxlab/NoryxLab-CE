package repository

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Repository struct {
	ID             string    `json:"id"`
	OwnerUserID    string    `json:"ownerUserId"`
	Name           string    `json:"name"`
	URL            string    `json:"url"`
	DefaultRef     string    `json:"defaultRef"`
	AuthSecretName string    `json:"authSecretName,omitempty"`
	AuthType       string    `json:"authType"`
	GitAuthorName  string    `json:"gitAuthorName,omitempty"`
	GitAuthorEmail string    `json:"gitAuthorEmail,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func New(ownerUserID, name, url, defaultRef, authSecretName, authType, gitAuthorName, gitAuthorEmail string) Repository {
	now := time.Now().UTC()
	return Repository{
		ID:             uuid.NewString(),
		OwnerUserID:    strings.TrimSpace(ownerUserID),
		Name:           strings.TrimSpace(name),
		URL:            strings.TrimSpace(url),
		DefaultRef:     strings.TrimSpace(defaultRef),
		AuthSecretName: strings.TrimSpace(authSecretName),
		AuthType:       strings.TrimSpace(authType),
		GitAuthorName:  strings.TrimSpace(gitAuthorName),
		GitAuthorEmail: strings.TrimSpace(gitAuthorEmail),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
