package memory

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/egress"
)

type EgressRuleStore struct {
	mu    sync.RWMutex
	items map[string]egress.Rule
}

func NewEgressRuleStore() *EgressRuleStore { return &EgressRuleStore{items: map[string]egress.Rule{}} }

func (s *EgressRuleStore) List() ([]egress.Rule, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]egress.Rule, 0, len(s.items))
	for _, item := range s.items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	return out, nil
}

func (s *EgressRuleStore) ListByProject(projectID string) ([]egress.Rule, error) {
	items, err := s.List()
	if err != nil {
		return nil, err
	}
	projectID = strings.TrimSpace(projectID)
	out := []egress.Rule{}
	for _, item := range items {
		if item.ProjectID == projectID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *EgressRuleStore) GetByID(id string) (egress.Rule, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.items[strings.TrimSpace(id)]
	return item, ok, nil
}

func (s *EgressRuleStore) Create(item egress.Rule) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[strings.TrimSpace(item.ID)] = item
	return nil
}

func (s *EgressRuleStore) UpdateDecision(id, status, reviewerID, note string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id = strings.TrimSpace(id)
	item, ok := s.items[id]
	if !ok {
		return nil
	}
	item.Status = strings.ToLower(strings.TrimSpace(status))
	item.ReviewerID = strings.TrimSpace(reviewerID)
	item.DecisionNote = strings.TrimSpace(note)
	item.UpdatedAt = time.Now().UTC()
	s.items[id] = item
	return nil
}
