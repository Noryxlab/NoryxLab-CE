package memory

import (
	"strings"
	"sync"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
)

type OntologyObjectStore struct {
	mu     sync.RWMutex
	items  []ontology.Ontology
	access []ontology.Access
}

func NewOntologyObjectStore() *OntologyObjectStore {
	return &OntologyObjectStore{items: []ontology.Ontology{}, access: []ontology.Access{}}
}

func (s *OntologyObjectStore) ListBySubjects(subjects []ontology.Subject) ([]ontology.Ontology, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []ontology.Ontology{}
	for _, item := range s.items {
		matchedOwner := false
		for _, subject := range subjects {
			if item.OwnerType == subject.Type && item.OwnerID == strings.TrimSpace(subject.ID) {
				item.AccessRole = "owner"
				out = append(out, item)
				matchedOwner = true
				break
			}
		}
		if matchedOwner {
			continue
		}
		best := ""
		for _, access := range s.access {
			for _, subject := range subjects {
				if access.OntologyID == item.ID && access.SubjectType == subject.Type && access.SubjectID == strings.TrimSpace(subject.ID) {
					if access.Role == "writer" || best == "" {
						best = access.Role
					}
				}
			}
		}
		if best != "" {
			item.AccessRole = best
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *OntologyObjectStore) ListAll() ([]ontology.Ontology, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]ontology.Ontology(nil), s.items...), nil
}

func (s *OntologyObjectStore) GetByID(id string) (ontology.Ontology, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.items {
		if item.ID == strings.TrimSpace(id) {
			return item, true, nil
		}
	}
	return ontology.Ontology{}, false, nil
}

func (s *OntologyObjectStore) Create(item ontology.Ontology) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
	return nil
}

func (s *OntologyObjectStore) UpdateMetadata(ontologyID, name, description string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == strings.TrimSpace(ontologyID) {
			s.items[i].Name = strings.TrimSpace(name)
			s.items[i].Description = strings.TrimSpace(description)
			s.items[i].UpdatedAt = time.Now().UTC()
		}
	}
	return nil
}

func (s *OntologyObjectStore) UpdateOwner(ontologyID, ownerType, ownerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == strings.TrimSpace(ontologyID) {
			s.items[i].OwnerType = strings.TrimSpace(ownerType)
			s.items[i].OwnerID = strings.TrimSpace(ownerID)
			if ownerType == "user" {
				s.items[i].OwnerUserID = strings.TrimSpace(ownerID)
			}
			s.items[i].UpdatedAt = time.Now().UTC()
		}
	}
	return nil
}

func (s *OntologyObjectStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	target := strings.TrimSpace(id)
	items := s.items[:0]
	for _, item := range s.items {
		if item.ID != target {
			items = append(items, item)
		}
	}
	s.items = items
	access := s.access[:0]
	for _, item := range s.access {
		if item.OntologyID != target {
			access = append(access, item)
		}
	}
	s.access = access
	return nil
}

func (s *OntologyObjectStore) ListAccess(ontologyID string) ([]ontology.Access, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []ontology.Access{}
	for _, item := range s.access {
		if item.OntologyID == strings.TrimSpace(ontologyID) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *OntologyObjectStore) GetAccess(ontologyID, subjectType, subjectID string) (ontology.Access, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.access {
		if item.OntologyID == strings.TrimSpace(ontologyID) && item.SubjectType == strings.TrimSpace(subjectType) && item.SubjectID == strings.TrimSpace(subjectID) {
			return item, true, nil
		}
	}
	return ontology.Access{}, false, nil
}

func (s *OntologyObjectStore) SetAccess(item ontology.Access) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.access {
		if s.access[i].OntologyID == item.OntologyID && s.access[i].SubjectType == item.SubjectType && s.access[i].SubjectID == item.SubjectID {
			s.access[i] = item
			return nil
		}
	}
	s.access = append(s.access, item)
	return nil
}

func (s *OntologyObjectStore) DeleteAccess(ontologyID, subjectType, subjectID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.access[:0]
	for _, item := range s.access {
		if item.OntologyID != strings.TrimSpace(ontologyID) || item.SubjectType != strings.TrimSpace(subjectType) || item.SubjectID != strings.TrimSpace(subjectID) {
			out = append(out, item)
		}
	}
	s.access = out
	return nil
}
