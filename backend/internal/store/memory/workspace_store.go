package memory

import (
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
)

type WorkspaceStore struct {
	mu    sync.RWMutex
	items []workspace.Workspace
}

func NewWorkspaceStore() *WorkspaceStore {
	return &WorkspaceStore{items: []workspace.Workspace{}}
}

func (s *WorkspaceStore) List() ([]workspace.Workspace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]workspace.Workspace, len(s.items))
	copy(out, s.items)
	return out, nil
}

func (s *WorkspaceStore) Create(w workspace.Workspace) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items = append(s.items, w)
	return nil
}
