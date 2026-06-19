package memory

import (
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	storepkg "github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

type AccessStore struct {
	mu      sync.RWMutex
	project map[string]map[string]access.Role
}

func NewAccessStore() *AccessStore {
	return &AccessStore{project: map[string]map[string]access.Role{}}
}

func (s *AccessStore) SetRole(projectID, userID string, role access.Role) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.project[projectID]; !ok {
		s.project[projectID] = map[string]access.Role{}
	}
	s.project[projectID][userID] = role
}

func (s *AccessStore) GetRole(projectID, userID string) (access.Role, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users, ok := s.project[projectID]
	if !ok {
		return "", false
	}

	role, ok := users[userID]
	return role, ok
}

func (s *AccessStore) ListProjectRoles() ([]storepkg.ProjectRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []storepkg.ProjectRole{}
	for projectID, users := range s.project {
		for userID, role := range users {
			out = append(out, storepkg.ProjectRole{ProjectID: projectID, UserID: userID, Role: role})
		}
	}
	return out, nil
}
