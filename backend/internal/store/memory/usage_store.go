package memory

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/usage"
)

type UsageStore struct {
	mu      sync.RWMutex
	samples []usage.Sample
}

func NewUsageStore() *UsageStore { return &UsageStore{} }

func (s *UsageStore) Record(samples []usage.Sample) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.samples = append(s.samples, samples...)
	return nil
}

func (s *UsageStore) ListByProject(projectID string, from, to time.Time) ([]usage.Sample, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	projectID = strings.TrimSpace(projectID)
	out := []usage.Sample{}
	for _, sample := range s.samples {
		if sample.ProjectID != projectID || sample.At.Before(from) || sample.At.After(to) {
			continue
		}
		out = append(out, sample)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
	return out, nil
}

func (s *UsageStore) ListProjects(from, to time.Time) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := map[string]struct{}{}
	for _, sample := range s.samples {
		if sample.At.Before(from) || sample.At.After(to) {
			continue
		}
		seen[sample.ProjectID] = struct{}{}
	}
	out := make([]string, 0, len(seen))
	for id := range seen {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

func (s *UsageStore) DeleteBefore(cutoff time.Time) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.samples[:0]
	var removed int64
	for _, sample := range s.samples {
		if sample.At.Before(cutoff) {
			removed++
			continue
		}
		kept = append(kept, sample)
	}
	s.samples = kept
	return removed, nil
}
