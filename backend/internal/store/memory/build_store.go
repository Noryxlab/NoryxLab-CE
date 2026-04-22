package memory

import (
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/build"
)

type BuildStore struct {
	mu    sync.RWMutex
	items []build.Build
}

func NewBuildStore() *BuildStore {
	return &BuildStore{items: []build.Build{}}
}

func (s *BuildStore) List() ([]build.Build, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]build.Build, len(s.items))
	copy(out, s.items)
	return out, nil
}

func (s *BuildStore) Create(b build.Build) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items = append(s.items, b)
	return nil
}
