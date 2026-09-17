package memory

import (
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

type DatasetSizeStore struct {
	mu    sync.RWMutex
	items map[string]store.DatasetSize
}

func NewDatasetSizeStore() *DatasetSizeStore {
	return &DatasetSizeStore{items: map[string]store.DatasetSize{}}
}

func (s *DatasetSizeStore) Upsert(entry store.DatasetSize) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[entry.DatasetID] = entry
	return nil
}

func (s *DatasetSizeStore) List() ([]store.DatasetSize, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sizes := make([]store.DatasetSize, 0, len(s.items))
	for _, entry := range s.items {
		sizes = append(sizes, entry)
	}
	return sizes, nil
}
