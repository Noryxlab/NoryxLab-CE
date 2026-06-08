package memory

import (
	"strings"
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
)

type DatasetStore struct {
	mu     sync.RWMutex
	items  []dataset.Dataset
	access []dataset.Access
}

func NewDatasetStore() *DatasetStore {
	return &DatasetStore{items: []dataset.Dataset{}, access: []dataset.Access{}}
}

func (s *DatasetStore) ListByUser(userID string) ([]dataset.Dataset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]dataset.Dataset, 0)
	for _, item := range s.items {
		if item.OwnerUserID == strings.TrimSpace(userID) {
			item.AccessRole = "owner"
			out = append(out, item)
			continue
		}
		for _, access := range s.access {
			if access.DatasetID == item.ID && access.UserID == strings.TrimSpace(userID) {
				item.AccessRole = access.Role
				out = append(out, item)
				break
			}
		}
	}
	return out, nil
}

func (s *DatasetStore) ListAll() ([]dataset.Dataset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]dataset.Dataset(nil), s.items...), nil
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

func (s *DatasetStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	target := strings.TrimSpace(id)
	filtered := make([]dataset.Dataset, 0, len(s.items))
	for _, item := range s.items {
		if item.ID != target {
			filtered = append(filtered, item)
		}
	}
	s.items = filtered
	access := s.access[:0]
	for _, item := range s.access {
		if item.DatasetID != target {
			access = append(access, item)
		}
	}
	s.access = access
	return nil
}

func (s *DatasetStore) ListAccess(datasetID string) ([]dataset.Access, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []dataset.Access{}
	for _, item := range s.access {
		if item.DatasetID == strings.TrimSpace(datasetID) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *DatasetStore) GetAccess(datasetID, userID string) (dataset.Access, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.access {
		if item.DatasetID == strings.TrimSpace(datasetID) && item.UserID == strings.TrimSpace(userID) {
			return item, true, nil
		}
	}
	return dataset.Access{}, false, nil
}

func (s *DatasetStore) SetAccess(item dataset.Access) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.access {
		if s.access[i].DatasetID == item.DatasetID && s.access[i].UserID == item.UserID {
			s.access[i] = item
			return nil
		}
	}
	s.access = append(s.access, item)
	return nil
}

func (s *DatasetStore) DeleteAccess(datasetID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.access[:0]
	for _, item := range s.access {
		if item.DatasetID != strings.TrimSpace(datasetID) || item.UserID != strings.TrimSpace(userID) {
			out = append(out, item)
		}
	}
	s.access = out
	return nil
}
