package memory

import (
	"strings"
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/job"
)

type JobStore struct {
	mu    sync.RWMutex
	items []job.Job
}

func NewJobStore() *JobStore {
	return &JobStore{items: []job.Job{}}
}

func (s *JobStore) List() ([]job.Job, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]job.Job, len(s.items))
	copy(out, s.items)
	return out, nil
}

func (s *JobStore) GetByID(id string) (job.Job, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.items {
		if item.ID == id {
			return item, true, nil
		}
	}
	return job.Job{}, false, nil
}

func (s *JobStore) Create(item job.Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
	return nil
}

func (s *JobStore) Upsert(item job.Job) error {
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

func (s *JobStore) Delete(id string) error {
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
