package project

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Project struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	OwnerType   string `json:"ownerType"`
	OwnerID     string `json:"ownerId"`
	// OwnerName is what a screen shows. An organization is stored by its
	// identifier, and 57c801a4-f273-4844-97c6-28307872a480 tells a reader
	// nothing; the platform knows the name, so it resolves it here rather
	// than making every screen ask separately - and an interface that cannot
	// list organizations, which a non-administrator cannot, would show the
	// identifier for ever.
	OwnerName string `json:"ownerName,omitempty"`
	// AdminVisible marks a project visible only through the platform admin
	// role, with no membership behind it.
	AdminVisible bool `json:"adminVisible,omitempty"`
	// WorkspaceStorageSize is the volume every workspace of this project gets,
	// as a Kubernetes quantity. Empty means the platform default: a project
	// that never sets it keeps following the installation.
	//
	// It lives here rather than on the launch form because it is a decision
	// about the project, not about the person starting a notebook - an Essilor
	// engineer asked to stop being shown "10 Go" when launching, and they were
	// right: capacity is infrastructure, and it was the same value every time.
	WorkspaceStorageSize string `json:"workspaceStorageSize,omitempty"`
	CanManageOwner       bool   `json:"canManageOwner,omitempty"`
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
