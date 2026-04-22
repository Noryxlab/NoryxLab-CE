package memory

import (
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
)

type ProjectStore struct {
	mu    sync.RWMutex
	items []project.Project
}

func NewProjectStore() *ProjectStore {
	return &ProjectStore{items: []project.Project{}}
}

func (s *ProjectStore) List() ([]project.Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]project.Project, len(s.items))
	copy(out, s.items)
	return out, nil
}

func (s *ProjectStore) Create(p project.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items = append(s.items, p)
	return nil
}
