package memory

import (
	"strings"
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/quota"
)

type QuotaStore struct {
	mu    sync.RWMutex
	items map[string]quota.Quota
}

func NewQuotaStore() *QuotaStore {
	return &QuotaStore{items: map[string]quota.Quota{}}
}

func (s *QuotaStore) Get(projectID string) (quota.Quota, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, found := s.items[strings.TrimSpace(projectID)]
	return item, found, nil
}

func (s *QuotaStore) List() ([]quota.Quota, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]quota.Quota, 0, len(s.items))
	for _, item := range s.items {
		out = append(out, item)
	}
	return out, nil
}

func (s *QuotaStore) Set(item quota.Quota) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item.ProjectID = strings.TrimSpace(item.ProjectID)
	// An empty quota is a removal rather than a row of zeroes: "no limit" and
	// "limit of zero" must never be the same thing in a store.
	if item.Empty() {
		delete(s.items, item.ProjectID)
		return nil
	}
	s.items[item.ProjectID] = item
	return nil
}

func (s *QuotaStore) Delete(projectID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, strings.TrimSpace(projectID))
	return nil
}
