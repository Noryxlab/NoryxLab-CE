package memory

import (
	"sort"
	"strings"
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workflow"
)

// WorkflowStore keeps definitions and runs in process, so the runner can be
// tested without Postgres. Same ordering as the durable one.
type WorkflowStore struct {
	mu    sync.RWMutex
	items map[string]workflow.Definition
	runs  map[string]workflow.Run
}

func NewWorkflowStore() *WorkflowStore {
	return &WorkflowStore{items: map[string]workflow.Definition{}, runs: map[string]workflow.Run{}}
}

func (s *WorkflowStore) ListByOwner(ownerUserID string) ([]workflow.Definition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []workflow.Definition{}
	for _, item := range s.items {
		if item.OwnerUserID == strings.TrimSpace(ownerUserID) {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *WorkflowStore) ListAll() ([]workflow.Definition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]workflow.Definition, 0, len(s.items))
	for _, item := range s.items {
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

func (s *WorkflowStore) GetByID(id string) (workflow.Definition, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, found := s.items[strings.TrimSpace(id)]
	return item, found, nil
}

func (s *WorkflowStore) Create(item workflow.Definition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
	return nil
}

func (s *WorkflowStore) Update(item workflow.Definition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[item.ID] = item
	return nil
}

func (s *WorkflowStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id = strings.TrimSpace(id)
	delete(s.items, id)
	for runID, run := range s.runs {
		if run.WorkflowID == id {
			delete(s.runs, runID)
		}
	}
	return nil
}

func (s *WorkflowStore) CreateRun(run workflow.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.ID] = run
	return nil
}

func (s *WorkflowStore) UpdateRun(run workflow.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runs[run.ID] = run
	return nil
}

func (s *WorkflowStore) GetRun(id string) (workflow.Run, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	run, found := s.runs[strings.TrimSpace(id)]
	return run, found, nil
}

func (s *WorkflowStore) ListRuns(workflowID string, limit int) ([]workflow.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []workflow.Run{}
	for _, run := range s.runs {
		if run.WorkflowID == strings.TrimSpace(workflowID) {
			out = append(out, run)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *WorkflowStore) ListUnfinishedRuns() ([]workflow.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []workflow.Run{}
	for _, run := range s.runs {
		if run.Status == workflow.StatusPending || run.Status == workflow.StatusRunning {
			out = append(out, run)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.Before(out[j].StartedAt) })
	return out, nil
}
