package memory

import (
	"strings"
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/organization"
)

// OrganizationStore keeps the administrators in memory, for the tests and for
// an installation running without a database.
type OrganizationStore struct {
	mu     sync.RWMutex
	admins []organization.Admin
}

func NewOrganizationStore() *OrganizationStore { return &OrganizationStore{} }

func (s *OrganizationStore) ListAdmins(organizationID string) ([]organization.Admin, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []organization.Admin{}
	for _, item := range s.admins {
		if strings.EqualFold(item.OrganizationID, strings.TrimSpace(organizationID)) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *OrganizationStore) AddAdmin(item organization.Admin) error {
	item.Normalise()
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, existing := range s.admins {
		if strings.EqualFold(existing.OrganizationID, item.OrganizationID) &&
			strings.EqualFold(existing.SubjectType, item.SubjectType) &&
			strings.EqualFold(existing.SubjectID, item.SubjectID) {
			return nil
		}
	}
	s.admins = append(s.admins, item)
	return nil
}

func (s *OrganizationStore) RemoveAdmin(organizationID, subjectType, subjectID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.admins[:0]
	for _, item := range s.admins {
		if strings.EqualFold(item.OrganizationID, strings.TrimSpace(organizationID)) &&
			strings.EqualFold(item.SubjectType, strings.TrimSpace(subjectType)) &&
			strings.EqualFold(item.SubjectID, strings.TrimSpace(subjectID)) {
			continue
		}
		kept = append(kept, item)
	}
	s.admins = kept
	return nil
}

func (s *OrganizationStore) AdministeredBy(userIDs []string, teamIDs []string) ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	vu := map[string]bool{}
	out := []string{}
	for _, item := range s.admins {
		candidats := userIDs
		if strings.EqualFold(item.SubjectType, "team") {
			candidats = teamIDs
		}
		for _, candidat := range candidats {
			if strings.TrimSpace(candidat) == "" || !strings.EqualFold(candidat, item.SubjectID) {
				continue
			}
			if !vu[item.OrganizationID] {
				vu[item.OrganizationID] = true
				out = append(out, item.OrganizationID)
			}
		}
	}
	return out, nil
}
