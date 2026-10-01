package memory

import (
	"database/sql"
	"strings"
	"sync"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/datasource"
)

type DatasourceStore struct {
	mu    sync.RWMutex
	items []datasource.Datasource
}

func NewDatasourceStore() *DatasourceStore {
	return &DatasourceStore{items: []datasource.Datasource{}}
}

func (s *DatasourceStore) ListByUser(userID string) ([]datasource.Datasource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	uid := strings.TrimSpace(userID)
	out := []datasource.Datasource{}
	for _, item := range s.items {
		if strings.TrimSpace(item.OwnerUserID) == uid {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *DatasourceStore) ListAll() ([]datasource.Datasource, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]datasource.Datasource(nil), s.items...)
	return out, nil
}

func (s *DatasourceStore) GetByID(id string) (datasource.Datasource, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id = strings.TrimSpace(id)
	for _, item := range s.items {
		if item.ID == id {
			return item, true, nil
		}
	}
	return datasource.Datasource{}, false, nil
}

func (s *DatasourceStore) Create(item datasource.Datasource) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
	return nil
}

func (s *DatasourceStore) Upsert(item datasource.Datasource) error {
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

func (s *DatasourceStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
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

// SetOwner hands the connector to somebody else, and touches nothing else.
func (s *DatasourceStore) SetOwner(id, ownerType, ownerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	trimmed := strings.TrimSpace(id)
	for i := range s.items {
		if s.items[i].ID != trimmed {
			continue
		}
		s.items[i].OwnerType = strings.TrimSpace(ownerType)
		s.items[i].OwnerID = strings.TrimSpace(ownerID)
		s.items[i].UpdatedAt = time.Now().UTC()
		return nil
	}
	return sql.ErrNoRows
}
