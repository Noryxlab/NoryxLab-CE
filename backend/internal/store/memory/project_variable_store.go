package memory

import (
	"sort"
	"strings"
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/projectvar"
)

type ProjectVariableStore struct {
	mu    sync.RWMutex
	items []projectvar.Variable
}

func NewProjectVariableStore() *ProjectVariableStore {
	return &ProjectVariableStore{items: []projectvar.Variable{}}
}

func (s *ProjectVariableStore) ListByProject(projectID string) ([]projectvar.Variable, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	projectID = strings.TrimSpace(projectID)
	out := make([]projectvar.Variable, 0)
	for _, item := range s.items {
		if item.ProjectID == projectID {
			out = append(out, item)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

func (s *ProjectVariableStore) ListAll() ([]projectvar.Variable, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]projectvar.Variable, len(s.items))
	copy(out, s.items)
	return out, nil
}

func (s *ProjectVariableStore) GetByName(projectID, name string) (projectvar.Variable, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.items {
		if item.ProjectID == strings.TrimSpace(projectID) && item.Name == strings.TrimSpace(name) {
			return item, true, nil
		}
	}
	return projectvar.Variable{}, false, nil
}

func (s *ProjectVariableStore) Upsert(item projectvar.Variable) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, existing := range s.items {
		if existing.ProjectID == item.ProjectID && existing.Name == item.Name {
			item.CreatedAt = existing.CreatedAt
			s.items[index] = item
			return nil
		}
	}
	s.items = append(s.items, item)
	return nil
}

func (s *ProjectVariableStore) Delete(projectID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.items[:0]
	for _, item := range s.items {
		if item.ProjectID == strings.TrimSpace(projectID) && item.Name == strings.TrimSpace(name) {
			continue
		}
		kept = append(kept, item)
	}
	s.items = kept
	return nil
}
