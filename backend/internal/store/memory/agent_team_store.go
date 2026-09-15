package memory

import (
	"sort"
	"strings"
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/agent"
)

type AgentTeamStore struct {
	mu       sync.RWMutex
	items    map[string]agent.Team
	mandates map[string]agent.Mandate
}

func NewAgentTeamStore() *AgentTeamStore {
	return &AgentTeamStore{items: map[string]agent.Team{}, mandates: map[string]agent.Mandate{}}
}

func (s *AgentTeamStore) ListByOwner(ownerUserID string) ([]agent.Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []agent.Team{}
	for _, item := range s.items {
		if item.OwnerUserID == strings.TrimSpace(ownerUserID) {
			out = append(out, item)
		}
	}
	// Newest last, so a list read twice does not reorder itself.
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *AgentTeamStore) GetByID(id string) (agent.Team, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, found := s.items[strings.TrimSpace(id)]
	return item, found, nil
}

func (s *AgentTeamStore) Create(item agent.Team) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
	return nil
}

func (s *AgentTeamStore) Update(item agent.Team) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
	return nil
}

func (s *AgentTeamStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, strings.TrimSpace(id))
	return nil
}

func (s *AgentTeamStore) ListMandates(teamID string) ([]agent.Mandate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []agent.Mandate{}
	for _, item := range s.mandates {
		if item.TeamID == strings.TrimSpace(teamID) {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *AgentTeamStore) CreateMandate(item agent.Mandate) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Same statement twice is the same statement, as in the durable store.
	for _, existing := range s.mandates {
		if existing.LeadID == item.LeadID && existing.MemberID == item.MemberID &&
			existing.Action == item.Action {
			return nil
		}
	}
	s.mandates[item.ID] = item
	return nil
}

func (s *AgentTeamStore) DeleteMandate(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.mandates, strings.TrimSpace(id))
	return nil
}
