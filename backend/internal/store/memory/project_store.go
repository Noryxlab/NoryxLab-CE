package memory

import (
	"sync"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
)

type ProjectStore struct {
	mu    sync.RWMutex
	items []project.Project
}

func NewProjectStore() *ProjectStore {
	return &ProjectStore{items: []project.Project{}}
}

func (s *ProjectStore) List() ([]project.Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]project.Project, len(s.items))
	copy(out, s.items)
	return out, nil
}

func (s *ProjectStore) Create(p project.Project) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items = append(s.items, p)
	return nil
}

func (s *ProjectStore) UpdateMetadata(projectID, name, description string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == projectID {
			s.items[i].Name = name
			s.items[i].Description = description
			s.items[i].UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	return nil
}

func (s *ProjectStore) UpdateOwner(projectID, ownerType, ownerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.items {
		if s.items[i].ID == projectID {
			s.items[i].OwnerType = ownerType
			s.items[i].OwnerID = ownerID
			return nil
		}
	}
	return nil
}

func (s *ProjectStore) DeleteProject(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := make([]project.Project, 0, len(s.items))
	for _, item := range s.items {
		if item.ID == id {
			continue
		}
		filtered = append(filtered, item)
	}
	s.items = filtered
	return nil
}
