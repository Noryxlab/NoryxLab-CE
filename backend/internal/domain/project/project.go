package project

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	OwnerType      string    `json:"ownerType"`
	OwnerID        string    `json:"ownerId"`
	CanManageOwner bool      `json:"canManageOwner,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

func New(name string) Project {
	return Project{
		ID:        uuid.NewString(),
		Name:      strings.TrimSpace(name),
		CreatedAt: time.Now().UTC(),
	}
}

func NewOwned(ownerUserID, name string) Project {
	item := New(name)
	item.OwnerType = "user"
	item.OwnerID = strings.TrimSpace(ownerUserID)
	return item
}
