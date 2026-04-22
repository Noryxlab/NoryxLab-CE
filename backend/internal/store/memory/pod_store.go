package memory

import (
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/pod"
)

type PodStore struct {
	mu    sync.RWMutex
	items []pod.Launch
}

func NewPodStore() *PodStore {
	return &PodStore{items: []pod.Launch{}}
}

func (s *PodStore) List() ([]pod.Launch, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]pod.Launch, len(s.items))
	copy(out, s.items)
	return out, nil
}

func (s *PodStore) Create(p pod.Launch) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items = append(s.items, p)
	return nil
}
