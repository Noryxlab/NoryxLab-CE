package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"

type ProjectRole struct {
	ProjectID string      `json:"projectId"`
	UserID    string      `json:"userId"`
	Role      access.Role `json:"role"`
}

type AccessStore interface {
	SetRole(projectID, userID string, role access.Role)
	GetRole(projectID, userID string) (access.Role, bool)
	ListProjectRoles() ([]ProjectRole, error)
}
