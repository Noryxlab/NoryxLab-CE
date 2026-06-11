package memory

import (
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/audit"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

type AuditStore struct {
	mu    sync.RWMutex
	items []audit.Event
}

func NewAuditStore() *AuditStore {
	return &AuditStore{items: []audit.Event{}}
}

func (s *AuditStore) Create(event audit.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, event)
	return nil
}

func (s *AuditStore) List(filter store.AuditFilter) ([]audit.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	limit := filter.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	out := make([]audit.Event, 0, limit)
	for _, item := range s.items {
		if filter.Since != nil && item.OccurredAt.Before(*filter.Since) {
			continue
		}
		if filter.Until != nil && item.OccurredAt.After(*filter.Until) {
			continue
		}
		if filter.Action != "" && item.Action != filter.Action {
			continue
		}
		if filter.ActorUserID != "" && item.ActorUserID != filter.ActorUserID {
			continue
		}
		if filter.ResourceID != "" && item.ResourceID != filter.ResourceID {
			continue
		}
		if filter.ProjectID != "" && item.ProjectID != filter.ProjectID {
			continue
		}
		out = append(out, item)
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	// reverse to newest first
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, nil
}
