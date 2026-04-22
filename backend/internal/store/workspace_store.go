package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"

type WorkspaceStore interface {
	List() ([]workspace.Workspace, error)
	Create(w workspace.Workspace) error
}
