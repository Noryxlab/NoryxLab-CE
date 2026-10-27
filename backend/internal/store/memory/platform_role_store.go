package memory

import (
	"strings"
	"sync"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/platformrole"
)

// PlatformRoleStore keeps custom rows and assignments in memory.
type PlatformRoleStore struct {
	mu          sync.RWMutex
	custom      map[string]platformrole.Role
	assignments map[string]platformrole.Assignment
}

func NewPlatformRoleStore() *PlatformRoleStore {
	return &PlatformRoleStore{custom: map[string]platformrole.Role{}, assignments: map[string]platformrole.Assignment{}}
}

func (s *PlatformRoleStore) ListCustom() ([]platformrole.Role, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]platformrole.Role, 0, len(s.custom))
	for _, item := range s.custom {
		out = append(out, item)
	}
	return out, nil
}

func (s *PlatformRoleStore) GetCustom(key string) (platformrole.Role, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.custom[strings.ToLower(strings.TrimSpace(key))]
	return item, ok, nil
}

func (s *PlatformRoleStore) UpsertCustom(role platformrole.Role) error {
	role.Normalise()
	role.Assignable = true
	role.UpdatedAt = time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.custom[role.Key]; ok {
		role.CreatedAt = existing.CreatedAt
	} else {
		role.CreatedAt = role.UpdatedAt
	}
	s.custom[role.Key] = role
	return nil
}

func (s *PlatformRoleStore) DeleteCustom(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.custom, strings.ToLower(strings.TrimSpace(key)))
	return nil
}

func (s *PlatformRoleStore) GetAssignment(userID string) (platformrole.Assignment, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.assignments[strings.ToLower(strings.TrimSpace(userID))]
	return item, ok, nil
}

func (s *PlatformRoleStore) SetAssignment(item platformrole.Assignment) error {
	if item.AssignedAt.IsZero() {
		item.AssignedAt = time.Now().UTC()
	}
	item.RoleKey = strings.ToLower(strings.TrimSpace(item.RoleKey))
	s.mu.Lock()
	defer s.mu.Unlock()
	s.assignments[strings.ToLower(strings.TrimSpace(item.UserID))] = item
	return nil
}

func (s *PlatformRoleStore) ClearAssignment(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.assignments, strings.ToLower(strings.TrimSpace(userID)))
	return nil
}

func (s *PlatformRoleStore) ListAssignments() ([]platformrole.Assignment, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]platformrole.Assignment, 0, len(s.assignments))
	for _, item := range s.assignments {
		out = append(out, item)
	}
	return out, nil
}
