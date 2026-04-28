package memory

import (
	"strings"
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/app"
)

type AppStore struct {
	mu    sync.RWMutex
	items []app.App
}

func NewAppStore() *AppStore {
	return &AppStore{items: []app.App{}}
}

func (s *AppStore) List() ([]app.App, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]app.App, len(s.items))
	copy(out, s.items)
	return out, nil
}

func (s *AppStore) GetByID(id string) (app.App, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id = strings.TrimSpace(id)
	for _, item := range s.items {
		if item.ID == id {
			return item, true, nil
		}
	}
	return app.App{}, false, nil
}

func (s *AppStore) GetBySlug(slug string) (app.App, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	slug = strings.TrimSpace(strings.ToLower(slug))
	for _, item := range s.items {
		if strings.EqualFold(strings.TrimSpace(item.Slug), slug) {
			return item, true, nil
		}
	}
	return app.App{}, false, nil
}

func (s *AppStore) Create(item app.App) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
	return nil
}

func (s *AppStore) Upsert(item app.App) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == item.ID {
			s.items[i] = item
			return nil
		}
	}
	s.items = append(s.items, item)
	return nil
}

func (s *AppStore) Delete(id string) error {
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
