package memory

import (
	"strings"
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/storageendpoint"
)

type StorageEndpointStore struct {
	mu    sync.RWMutex
	items []storageendpoint.Endpoint
}

func NewStorageEndpointStore() *StorageEndpointStore {
	return &StorageEndpointStore{items: []storageendpoint.Endpoint{}}
}

func (s *StorageEndpointStore) List() ([]storageendpoint.Endpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]storageendpoint.Endpoint(nil), s.items...)
	return out, nil
}

func (s *StorageEndpointStore) GetByID(id string) (storageendpoint.Endpoint, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id = strings.TrimSpace(id)
	for _, item := range s.items {
		if item.ID == id {
			return item, true, nil
		}
	}
	return storageendpoint.Endpoint{}, false, nil
}

func (s *StorageEndpointStore) Create(item storageendpoint.Endpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
	return nil
}

func (s *StorageEndpointStore) Update(item storageendpoint.Endpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, current := range s.items {
		if current.ID == item.ID {
			s.items[i] = item
			return nil
		}
	}
	s.items = append(s.items, item)
	return nil
}

func (s *StorageEndpointStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id = strings.TrimSpace(id)
	out := s.items[:0]
	for _, item := range s.items {
		if item.ID == id {
			continue
		}
		out = append(out, item)
	}
	s.items = out
	return nil
}
