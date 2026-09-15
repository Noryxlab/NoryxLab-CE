package memory

import (
	"sort"
	"strings"
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/agent"
)

// AgentStore keeps agents and their runs in process.
//
// The durable one arrived without this, so anything reasoning about agents had
// to be tested against Postgres or not at all - and "or not at all" is what
// happened.
type AgentStore struct {
	mu    sync.RWMutex
	items map[string]agent.Agent
	runs  map[string][]agent.Run
}

func NewAgentStore() *AgentStore {
	return &AgentStore{items: map[string]agent.Agent{}, runs: map[string][]agent.Run{}}
}

func (s *AgentStore) ListByOwner(ownerUserID string) ([]agent.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []agent.Agent{}
	for _, item := range s.items {
		if item.OwnerUserID == strings.TrimSpace(ownerUserID) {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *AgentStore) ListAll() ([]agent.Agent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]agent.Agent, 0, len(s.items))
	for _, item := range s.items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *AgentStore) GetByID(id string) (agent.Agent, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, found := s.items[strings.TrimSpace(id)]
	return item, found, nil
}

func (s *AgentStore) Create(item agent.Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
	return nil
}

func (s *AgentStore) Update(item agent.Agent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
	return nil
}

func (s *AgentStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id = strings.TrimSpace(id)
	delete(s.items, id)
	delete(s.runs, id)
	return nil
}

func (s *AgentStore) AppendRun(run agent.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.AgentID] = append(s.runs[run.AgentID], run)
	return nil
}

func (s *AgentStore) UpdateRun(run agent.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, existing := range s.runs[run.AgentID] {
		if existing.ID == run.ID {
			s.runs[run.AgentID][index] = run
			return nil
		}
	}
	return nil
}

// ListRuns returns the most recent first, like the durable store, so a caller
// reading a page of history sees the same order either way.
func (s *AgentStore) ListRuns(agentID string, limit int) ([]agent.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	stored := s.runs[strings.TrimSpace(agentID)]
	out := make([]agent.Run, len(stored))
	copy(out, stored)
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}
