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

func (s *WorkspaceStore) GetByID(id string) (workspace.Workspace, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, item := range s.items {
		if item.ID == id {
			return item, true, nil
		}
	}

	return workspace.Workspace{}, false, nil
}

func (s *WorkspaceStore) Create(w workspace.Workspace) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items = append(s.items, w)
	return nil
}

func (s *WorkspaceStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, item := range s.items {
		if item.ID != id {
			continue
		}
		s.items = append(s.items[:i], s.items[i+1:]...)
		break
	}
	return nil
}
