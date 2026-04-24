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

func (s *BuildStore) GetByID(id string) (build.Build, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, item := range s.items {
		if item.ID == id {
			return item, true, nil
		}
	}
	return build.Build{}, false, nil
}

func (s *BuildStore) Create(b build.Build) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items = append(s.items, b)
	return nil
}

func (s *BuildStore) Upsert(b build.Build) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, item := range s.items {
		if item.ID == b.ID {
			s.items[i] = b
			return nil
		}
	}
	s.items = append(s.items, b)
	return nil
}
