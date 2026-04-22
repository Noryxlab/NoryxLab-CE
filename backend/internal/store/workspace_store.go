package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"

type WorkspaceStore interface {
	List() ([]workspace.Workspace, error)
	GetByID(id string) (workspace.Workspace, bool, error)
	Create(w workspace.Workspace) error
	Delete(id string) error
}
