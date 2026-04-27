package dataset

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Dataset struct {
	ID          string    `json:"id"`
	OwnerUserID string    `json:"ownerUserId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Bucket      string    `json:"bucket"`
	Prefix      string    `json:"prefix"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

func New(ownerUserID, name, description, bucket, prefix string) Dataset {
	now := time.Now().UTC()
	return Dataset{
		ID:          uuid.NewString(),
		OwnerUserID: strings.TrimSpace(ownerUserID),
		Name:        strings.TrimSpace(name),
		Description: strings.TrimSpace(description),
		Bucket:      strings.TrimSpace(bucket),
		Prefix:      strings.Trim(strings.TrimSpace(prefix), "/"),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}
