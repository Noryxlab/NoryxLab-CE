package memory

import (
	"strings"
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/cohort"
)

type CohortStore struct {
	mu      sync.RWMutex
	items   []cohort.Cohort
	members map[string][]cohort.Member
}

func NewCohortStore() *CohortStore {
	return &CohortStore{items: []cohort.Cohort{}, members: map[string][]cohort.Member{}}
}

func (s *CohortStore) ListByProject(projectID string) ([]cohort.Cohort, error) {
	return s.filter(func(item cohort.Cohort) bool { return item.ProjectID == strings.TrimSpace(projectID) }), nil
}

func (s *CohortStore) ListByOntology(ontologyID string) ([]cohort.Cohort, error) {
	return s.filter(func(item cohort.Cohort) bool { return item.OntologyID == strings.TrimSpace(ontologyID) }), nil
}

func (s *CohortStore) filter(keep func(cohort.Cohort) bool) []cohort.Cohort {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []cohort.Cohort{}
	for _, item := range s.items {
		if keep(item) {
			out = append(out, item)
		}
	}
	return out
}

func (s *CohortStore) GetByID(id string) (cohort.Cohort, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.items {
		if item.ID == strings.TrimSpace(id) {
			return item, true, nil
		}
	}
	return cohort.Cohort{}, false, nil
}

func (s *CohortStore) Create(item cohort.Cohort, members []cohort.Member) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
	if s.members == nil {
		s.members = map[string][]cohort.Member{}
	}
	s.members[item.ID] = append([]cohort.Member(nil), members...)
	return nil
}

func (s *CohortStore) ListMembers(cohortID string, limit int) ([]cohort.Member, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	members := s.members[strings.TrimSpace(cohortID)]
	if limit > 0 && len(members) > limit {
		members = members[:limit]
	}
	return append([]cohort.Member(nil), members...), nil
}

func (s *CohortStore) Delete(id string) error {
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
