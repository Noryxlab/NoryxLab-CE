package memory

import (
	"strings"
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
)

type DatasetStore struct {
	mu    sync.RWMutex
	items []dataset.Dataset
}

func NewDatasetStore() *DatasetStore {
	return &DatasetStore{items: []dataset.Dataset{}}
}

func (s *DatasetStore) ListByUser(userID string) ([]dataset.Dataset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]dataset.Dataset, 0)
	for _, item := range s.items {
		if item.OwnerUserID == strings.TrimSpace(userID) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *DatasetStore) GetByID(id string) (dataset.Dataset, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.items {
		if item.ID == strings.TrimSpace(id) {
			return item, true, nil
		}
	}
	return dataset.Dataset{}, false, nil
}

func (s *DatasetStore) Create(item dataset.Dataset) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
	return nil
}
