package memory

import (
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/team"
	storepkg "github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

// TeamStore holds teams in memory.
//
// It exists so the handlers can be tested without a database, and so an
// installation running without Postgres keeps a working platform rather than a
// half-disabled one. It answers the same refusals as the real store - a
// duplicate name is a duplicate name here too - because a test that only
// passes against the lenient implementation proves nothing about production.
type TeamStore struct {
	mu      sync.RWMutex
	teams   map[string]team.Team
	members map[string]map[string]time.Time // team -> user -> joined
	grants  map[string]map[string]access.Role
}

func NewTeamStore() *TeamStore {
	return &TeamStore{
		teams:   map[string]team.Team{},
		members: map[string]map[string]time.Time{},
		grants:  map[string]map[string]access.Role{},
	}
}

func (s *TeamStore) ListByOrganization(organizationID string) ([]team.Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	organizationID = strings.TrimSpace(organizationID)
	out := []team.Team{}
	for _, item := range s.teams {
		if item.OrganizationID != organizationID {
			continue
		}
		item.MemberCount = len(s.members[item.ID])
		out = append(out, item)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func (s *TeamStore) GetByID(id string) (team.Team, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	item, ok := s.teams[strings.TrimSpace(id)]
	return item, ok, nil
}

func (s *TeamStore) Create(item team.Team) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.nameTaken(item.OrganizationID, item.Name, "") {
		return team.ErrNameTaken
	}
	s.teams[item.ID] = item
	return nil
}

func (s *TeamStore) Update(item team.Team) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.teams[item.ID]
	if !ok {
		return team.ErrNotFound
	}
	if s.nameTaken(existing.OrganizationID, item.Name, item.ID) {
		return team.ErrNameTaken
	}
	// The organization is not the caller's to change here: moving a team
	// between organizations would carry its grants with it, and that is a
	// different operation from a rename.
	existing.Name, existing.Description, existing.UpdatedAt = item.Name, item.Description, item.UpdatedAt
	s.teams[item.ID] = existing
	return nil
}

// nameTaken applies the same rule as the database index: unique within the
// organization, compared without case.
func (s *TeamStore) nameTaken(organizationID, name, exceptID string) bool {
	wanted := strings.ToLower(strings.TrimSpace(name))
	for id, item := range s.teams {
		if id == exceptID || item.OrganizationID != strings.TrimSpace(organizationID) {
			continue
		}
		if strings.ToLower(item.Name) == wanted {
			return true
		}
	}
	return false
}

func (s *TeamStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	id = strings.TrimSpace(id)
	delete(s.teams, id)
	delete(s.members, id)
	for _, byTeam := range s.grants {
		delete(byTeam, id)
	}
	return nil
}

func (s *TeamStore) ListMembers(teamID string) ([]team.Member, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	teamID = strings.TrimSpace(teamID)
	out := []team.Member{}
	for userID, joined := range s.members[teamID] {
		out = append(out, team.Member{TeamID: teamID, UserID: userID, JoinedAt: joined})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].JoinedAt.Before(out[j].JoinedAt) })
	return out, nil
}

func (s *TeamStore) AddMember(teamID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	teamID, userID = strings.TrimSpace(teamID), strings.TrimSpace(userID)
	if _, ok := s.members[teamID]; !ok {
		s.members[teamID] = map[string]time.Time{}
	}
	// Keeps the original date, like the database does: it answers when this
	// person gained what the team grants.
	if _, already := s.members[teamID][userID]; !already {
		s.members[teamID][userID] = time.Now().UTC()
	}
	return nil
}

func (s *TeamStore) RemoveMember(teamID, userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.members[strings.TrimSpace(teamID)], strings.TrimSpace(userID))
	return nil
}

func (s *TeamStore) Memberships() (map[string][]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[string][]string{}
	for teamID, members := range s.members {
		item, found := s.teams[teamID]
		if !found {
			continue
		}
		for userID := range members {
			key := strings.ToLower(strings.TrimSpace(userID))
			out[key] = append(out[key], item.Name)
		}
	}
	for _, names := range out {
		sort.Slice(names, func(i, j int) bool { return strings.ToLower(names[i]) < strings.ToLower(names[j]) })
	}
	return out, nil
}

func (s *TeamStore) ListByUser(userID string) ([]team.Team, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	userID = strings.TrimSpace(userID)
	out := []team.Team{}
	for teamID, people := range s.members {
		if _, ok := people[userID]; !ok {
			continue
		}
		if item, ok := s.teams[teamID]; ok {
			out = append(out, item)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

func (s *TeamStore) SetProjectRole(projectID, teamID string, role access.Role) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	projectID, teamID = strings.TrimSpace(projectID), strings.TrimSpace(teamID)
	if strings.TrimSpace(string(role)) == "" {
		delete(s.grants[projectID], teamID)
		return nil
	}
	if _, ok := s.grants[projectID]; !ok {
		s.grants[projectID] = map[string]access.Role{}
	}
	s.grants[projectID][teamID] = role
	return nil
}

func (s *TeamStore) ListProjectRoles(projectID string) ([]storepkg.ProjectTeamRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.grantsFor(strings.TrimSpace(projectID), ""), nil
}

func (s *TeamStore) ListProjectRolesForUser(projectID, userID string) ([]storepkg.ProjectTeamRole, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.grantsFor(strings.TrimSpace(projectID), strings.TrimSpace(userID)), nil
}

// grantsFor collects a project's team grants, optionally narrowed to the teams
// one person belongs to. Caller holds the lock.
func (s *TeamStore) grantsFor(projectID, userID string) []storepkg.ProjectTeamRole {
	out := []storepkg.ProjectTeamRole{}
	for teamID, role := range s.grants[projectID] {
		if userID != "" {
			if _, member := s.members[teamID][userID]; !member {
				continue
			}
		}
		out = append(out, storepkg.ProjectTeamRole{
			ProjectID: projectID, TeamID: teamID, Role: role,
			TeamName: s.teams[teamID].Name,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].TeamName+out[i].TeamID) <
			strings.ToLower(out[j].TeamName+out[j].TeamID)
	})
	return out
}
