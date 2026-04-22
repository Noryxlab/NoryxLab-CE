package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"

type AccessStore interface {
	SetRole(projectID, userID string, role access.Role)
	GetRole(projectID, userID string) (access.Role, bool)
}
