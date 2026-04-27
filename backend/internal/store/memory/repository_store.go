package memory

import (
	"strings"
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/repository"
)

type RepositoryStore struct {
	mu    sync.RWMutex
	items []repository.Repository
}

func NewRepositoryStore() *RepositoryStore {
	return &RepositoryStore{items: []repository.Repository{}}
}

func (s *RepositoryStore) ListByUser(userID string) ([]repository.Repository, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]repository.Repository, 0)
	for _, item := range s.items {
		if item.OwnerUserID == strings.TrimSpace(userID) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *RepositoryStore) GetByID(id string) (repository.Repository, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.items {
		if item.ID == strings.TrimSpace(id) {
			return item, true, nil
		}
	}
	return repository.Repository{}, false, nil
}

func (s *RepositoryStore) Create(item repository.Repository) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
	return nil
}
