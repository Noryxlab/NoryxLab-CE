package project

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	OwnerType      string `json:"ownerType"`
	OwnerID        string `json:"ownerId"`
	CanManageOwner bool   `json:"canManageOwner,omitempty"`
	// Role is the caller's effective role on this project, personal and
	// organization grants combined.
	Role string `json:"role,omitempty"`
	// CanManageMembers says whether the caller may change membership. Derived
	// here rather than in the interface, which would have to re-implement the
	// rule and would eventually disagree with the backend that enforces it.
	CanManageMembers  bool      `json:"canManageMembers,omitempty"`
	CreatedAt         time.Time `json:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt"`
	LastActivityAt    time.Time `json:"lastActivityAt"`
	RunningApps       int       `json:"runningApps"`
	RunningJobs       int       `json:"runningJobs"`
	RunningWorkspaces int       `json:"runningWorkspaces"`
}

func New(name, description string) Project {
	now := time.Now().UTC()
	return Project{
		ID:          uuid.NewString(),
		Name:        strings.TrimSpace(name),
		Description: strings.TrimSpace(description),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func NewOwned(ownerUserID, name, description string) Project {
	item := New(name, description)
	item.OwnerType = "user"
	item.OwnerID = strings.TrimSpace(ownerUserID)
	return item
}
