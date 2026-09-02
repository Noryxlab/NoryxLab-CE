package memory

import (
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/health"
)

type HealthEventStore struct {
	mu     sync.RWMutex
	events []health.Event
}

func NewHealthEventStore() *HealthEventStore { return &HealthEventStore{} }

func (s *HealthEventStore) Raise(event health.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// An already-open condition is not raised twice. The watcher forgets what
	// it announced when the process restarts, so without this every restart
	// would fork a new interval and the history would claim the platform
	// recovered and relapsed at each deployment.
	for _, existing := range s.events {
		if existing.Key == event.Key && existing.ResolvedAt == nil {
			return nil
		}
	}
	event.ID = uuid.NewString()
	if event.RaisedAt.IsZero() {
		event.RaisedAt = time.Now().UTC()
	}
	s.events = append(s.events, event)
	return nil
}

func (s *HealthEventStore) Resolve(key string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	stamp := at.UTC()
	for index := range s.events {
		if s.events[index].Key == key && s.events[index].ResolvedAt == nil {
			s.events[index].ResolvedAt = &stamp
		}
	}
	return nil
}

func (s *HealthEventStore) Open() ([]health.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []health.Event{}
	for _, event := range s.events {
		if event.ResolvedAt == nil {
			out = append(out, event)
		}
	}
	return out, nil
}

func (s *HealthEventStore) List(since time.Time, limit int) ([]health.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []health.Event{}
	for _, event := range s.events {
		if event.RaisedAt.Before(since) {
			continue
		}
		out = append(out, event)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].RaisedAt.After(out[j].RaisedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *HealthEventStore) Purge(before time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := make([]health.Event, 0, len(s.events))
	removed := 0
	for _, event := range s.events {
		// Open conditions are never purged however old: an alert that has been
		// firing for four months is the most important row in the table.
		if event.ResolvedAt != nil && event.ResolvedAt.Before(before) {
			removed++
			continue
		}
		kept = append(kept, event)
	}
	s.events = kept
	return removed, nil
}
