package memory

import (
	"sort"
	"strings"
	"sync"
)

type ProjectResourceStore struct {
	mu                 sync.RWMutex
	projectDatasets    map[string]map[string]struct{}
	projectRepos       map[string]map[string]struct{}
	projectDatasources map[string]map[string]struct{}
	projectOntologies  map[string]map[string]struct{}
	projectExtracts    map[string]map[string]struct{}
}

func NewProjectResourceStore() *ProjectResourceStore {
	return &ProjectResourceStore{
		projectDatasets:    map[string]map[string]struct{}{},
		projectRepos:       map[string]map[string]struct{}{},
		projectDatasources: map[string]map[string]struct{}{},
		projectOntologies:  map[string]map[string]struct{}{},
		projectExtracts:    map[string]map[string]struct{}{},
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

func (s *ProjectResourceStore) ListDatasetProjectIDs(datasetID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	wanted := strings.TrimSpace(datasetID)
	out := []string{}
	for projectID, datasets := range s.projectDatasets {
		if _, ok := datasets[wanted]; ok {
			out = append(out, projectID)
		}
	}
	sort.Strings(out)
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

func (s *ProjectResourceStore) AttachDatasource(projectID, datasourceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := strings.TrimSpace(projectID)
	d := strings.TrimSpace(datasourceID)
	if _, ok := s.projectDatasources[p]; !ok {
		s.projectDatasources[p] = map[string]struct{}{}
	}
	s.projectDatasources[p][d] = struct{}{}
	return nil
}

func (s *ProjectResourceStore) DetachDatasource(projectID, datasourceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := strings.TrimSpace(projectID)
	d := strings.TrimSpace(datasourceID)
	if m, ok := s.projectDatasources[p]; ok {
		delete(m, d)
	}
	return nil
}

func (s *ProjectResourceStore) ListProjectDatasourceIDs(projectID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := strings.TrimSpace(projectID)
	m := s.projectDatasources[p]
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	return out, nil
}

func (s *ProjectResourceStore) ListDatasourceProjectIDs(datasourceID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	d := strings.TrimSpace(datasourceID)
	out := []string{}
	for projectID, items := range s.projectDatasources {
		if _, ok := items[d]; ok {
			out = append(out, projectID)
		}
	}
	return out, nil
}

func (s *ProjectResourceStore) AttachExtract(projectID, extractID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := strings.TrimSpace(projectID)
	e := strings.TrimSpace(extractID)
	if _, ok := s.projectExtracts[p]; !ok {
		s.projectExtracts[p] = map[string]struct{}{}
	}
	s.projectExtracts[p][e] = struct{}{}
	return nil
}

func (s *ProjectResourceStore) DetachExtract(projectID, extractID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.projectExtracts[strings.TrimSpace(projectID)], strings.TrimSpace(extractID))
	return nil
}

func (s *ProjectResourceStore) ListProjectExtractIDs(projectID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []string{}
	for id := range s.projectExtracts[strings.TrimSpace(projectID)] {
		out = append(out, id)
	}
	sort.Strings(out)
	return out, nil
}

func (s *ProjectResourceStore) ListExtractProjectIDs(extractID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	wanted := strings.TrimSpace(extractID)
	out := []string{}
	for projectID, extracts := range s.projectExtracts {
		if _, ok := extracts[wanted]; ok {
			out = append(out, projectID)
		}
	}
	sort.Strings(out)
	return out, nil
}

func (s *ProjectResourceStore) AttachOntology(projectID, ontologyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := strings.TrimSpace(projectID)
	o := strings.TrimSpace(ontologyID)
	if _, ok := s.projectOntologies[p]; !ok {
		s.projectOntologies[p] = map[string]struct{}{}
	}
	s.projectOntologies[p][o] = struct{}{}
	return nil
}

func (s *ProjectResourceStore) DetachOntology(projectID, ontologyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := strings.TrimSpace(projectID)
	o := strings.TrimSpace(ontologyID)
	if m, ok := s.projectOntologies[p]; ok {
		delete(m, o)
	}
	return nil
}

func (s *ProjectResourceStore) ListProjectOntologyIDs(projectID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p := strings.TrimSpace(projectID)
	m := s.projectOntologies[p]
	out := make([]string, 0, len(m))
	for id := range m {
		out = append(out, id)
	}
	return out, nil
}

func (s *ProjectResourceStore) ListOntologyProjectIDs(ontologyID string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	wanted := strings.TrimSpace(ontologyID)
	out := []string{}
	for projectID, ontologies := range s.projectOntologies {
		if _, linked := ontologies[wanted]; linked {
			out = append(out, projectID)
		}
	}
	sort.Strings(out)
	return out, nil
}
