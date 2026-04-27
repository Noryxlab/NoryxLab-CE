package secret

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Secret struct {
	ID             string    `json:"id"`
	UserID         string    `json:"userId"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	ValueEncrypted string    `json:"-"`
	ExpiresAt      *time.Time `json:"expiresAt,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

func New(userID, name, secretType, valueEncrypted string) Secret {
	now := time.Now().UTC()
	name = strings.TrimSpace(name)
	secretType = strings.TrimSpace(secretType)
	if secretType == "" {
		secretType = "generic"
	}
	return Secret{
		ID:             uuid.NewString(),
		UserID:         strings.TrimSpace(userID),
		Name:           name,
		Type:           secretType,
		ValueEncrypted: valueEncrypted,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
