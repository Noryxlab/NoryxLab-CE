package memory

import (
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	storepkg "github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

type AccessStore struct {
	mu           sync.RWMutex
	project      map[string]map[string]access.Role
	organization map[string]map[string]access.Role
}

func NewAccessStore() *AccessStore {
	return &AccessStore{
		project:      map[string]map[string]access.Role{},
		organization: map[string]map[string]access.Role{},
	}
}

func (s *AccessStore) SetOrganizationRole(projectID, organizationID string, role access.Role) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if role == "" {
		delete(s.organization[projectID], organizationID)
		return nil
	}
	if _, ok := s.organization[projectID]; !ok {
		s.organization[projectID] = map[string]access.Role{}
	}
	s.organization[projectID][organizationID] = role
	return nil
}

func (s *AccessStore) ListOrganizationRoles(projectID string) ([]storepkg.ProjectOrganizationRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []storepkg.ProjectOrganizationRole{}
	for organizationID, role := range s.organization[projectID] {
		out = append(out, storepkg.ProjectOrganizationRole{
			ProjectID: projectID, OrganizationID: organizationID, Role: role,
		})
	}
	return out, nil
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
