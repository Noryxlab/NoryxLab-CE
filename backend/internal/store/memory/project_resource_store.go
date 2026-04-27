package memory

import (
	"strings"
	"sync"
)

type ProjectResourceStore struct {
	mu               sync.RWMutex
	projectDatasets  map[string]map[string]struct{}
	projectRepos     map[string]map[string]struct{}
}

func NewProjectResourceStore() *ProjectResourceStore {
	return &ProjectResourceStore{
		projectDatasets: map[string]map[string]struct{}{},
		projectRepos:    map[string]map[string]struct{}{},
	}
}

func (s *ProjectResourceStore) AttachDataset(projectID, datasetID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := strings.TrimSpace(projectID)
	d := strings.TrimSpace(datasetID)
	if _, ok := s.projectDatasets[p]; !ok {
		s.projectDatasets[p] = map[string]struct{}{}
	}
	s.projectDatasets[p][d] = struct{}{}
	return nil
}

func (s *ProjectResourceStore) DetachDataset(projectID, datasetID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := strings.TrimSpace(projectID)
	d := strings.TrimSpace(datasetID)
	if m, ok := s.projectDatasets[p]; ok {
		delete(m, d)
	}
	return nil
}

func (s *ProjectResourceStore) ListProjectDatasetIDs(projectID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := strings.TrimSpace(projectID)
	m := s.projectDatasets[p]
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	return out, nil
}

func (s *ProjectResourceStore) AttachRepository(projectID, repositoryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := strings.TrimSpace(projectID)
	r := strings.TrimSpace(repositoryID)
	if _, ok := s.projectRepos[p]; !ok {
		s.projectRepos[p] = map[string]struct{}{}
	}
	s.projectRepos[p][r] = struct{}{}
	return nil
}

func (s *ProjectResourceStore) DetachRepository(projectID, repositoryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := strings.TrimSpace(projectID)
	r := strings.TrimSpace(repositoryID)
	if m, ok := s.projectRepos[p]; ok {
		delete(m, r)
	}
	return nil
}

func (s *ProjectResourceStore) ListProjectRepositoryIDs(projectID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := strings.TrimSpace(projectID)
	m := s.projectRepos[p]
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	return out, nil
}
