package memory

import (
	"strings"
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
)

type ExtractStore struct {
	mu      sync.RWMutex
	items   []extract.Extract
	members map[string][]extract.Member
}

func NewExtractStore() *ExtractStore {
	return &ExtractStore{items: []extract.Extract{}, members: map[string][]extract.Member{}}
}

func (s *ExtractStore) ListByProject(projectID string) ([]extract.Extract, error) {
	return s.filter(func(item extract.Extract) bool { return item.ProjectID == strings.TrimSpace(projectID) }), nil
}

func (s *ExtractStore) ListByOntology(ontologyID string) ([]extract.Extract, error) {
	return s.filter(func(item extract.Extract) bool { return item.OntologyID == strings.TrimSpace(ontologyID) }), nil
}

func (s *ExtractStore) filter(keep func(extract.Extract) bool) []extract.Extract {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []extract.Extract{}
	for _, item := range s.items {
		if keep(item) {
			out = append(out, item)
		}
	}
	return out
}

func (s *ExtractStore) GetByID(id string) (extract.Extract, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.items {
		if item.ID == strings.TrimSpace(id) {
			return item, true, nil
		}
	}
	return extract.Extract{}, false, nil
}

func (s *ExtractStore) Create(item extract.Extract, members []extract.Member) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
	if s.members == nil {
		s.members = map[string][]extract.Member{}
	}
	s.members[item.ID] = append([]extract.Member(nil), members...)
	return nil
}

func (s *ExtractStore) ListMembers(extractID string, limit int) ([]extract.Member, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	members := s.members[strings.TrimSpace(extractID)]
	if limit > 0 && len(members) > limit {
		members = members[:limit]
	}
	return append([]extract.Member(nil), members...), nil
}

func (s *ExtractStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	trimmed := strings.TrimSpace(id)
	kept := s.items[:0]
	for _, item := range s.items {
		if item.ID != trimmed {
			kept = append(kept, item)
		}
	}
	s.items = kept
	delete(s.members, trimmed)
	return nil
}
