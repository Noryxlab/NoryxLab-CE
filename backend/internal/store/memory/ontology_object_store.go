package memory

import (
	"strings"
	"sync"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
)

type OntologyObjectStore struct {
	mu      sync.RWMutex
	items   []ontology.Ontology
	access  []ontology.Access
	objects map[string][]ontology.Object
	scans   map[string][]ontology.Scan
}

func NewOntologyObjectStore() *OntologyObjectStore {
	return &OntologyObjectStore{
		items:   []ontology.Ontology{},
		access:  []ontology.Access{},
		objects: map[string][]ontology.Object{},
		scans:   map[string][]ontology.Scan{},
	}
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

func (s *OntologyObjectStore) ReplaceManifest(ontologyID string, manifest []byte, generatedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == strings.TrimSpace(ontologyID) {
			s.items[i].Manifest = manifest
			if strings.TrimSpace(s.items[i].OwnerUserID) == "" {
				s.items[i].OwnerUserID = strings.TrimSpace(generatedBy)
			}
			s.items[i].UpdatedAt = time.Now().UTC()
		}
	}
	return nil
}

func (s *OntologyObjectStore) SetCard(ontologyID string, card *ontology.Card) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == strings.TrimSpace(ontologyID) {
			s.items[i].Card = card
			s.items[i].UpdatedAt = time.Now().UTC()
			return nil
		}
	}
	// Silencieux comme ReplaceManifest a cote : ce store sert les tests, et
	// c'est le handler qui a deja verifie que l'ontologie existe.
	return nil
}

func (s *OntologyObjectStore) SetStructureScan(ontologyID string, scan *ontology.StructureScan) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == strings.TrimSpace(ontologyID) {
			s.items[i].Structure = scan
			s.items[i].UpdatedAt = time.Now().UTC()
			return nil
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
	// La liste de fichiers part avec l'ontologie, comme en Postgres : la
	// laisser derriere soi orpheline etait le defaut, et un store de test qui
	// ne le reproduit pas ne protege de rien.
	delete(s.objects, target)
	delete(s.scans, target)
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

// Les photographies, la plus recente en tete, et on elague comme en Postgres.
func (s *OntologyObjectStore) AppendOntologyScan(scan ontology.Scan) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.scans == nil {
		s.scans = map[string][]ontology.Scan{}
	}
	id := strings.TrimSpace(scan.OntologyID)
	s.scans[id] = append([]ontology.Scan{scan}, s.scans[id]...)
	if len(s.scans[id]) > ontology.ScanHistoryKept {
		s.scans[id] = s.scans[id][:ontology.ScanHistoryKept]
	}
	return nil
}

func (s *OntologyObjectStore) ListOntologyScans(ontologyID string) ([]ontology.Scan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]ontology.Scan(nil), s.scans[strings.TrimSpace(ontologyID)]...), nil
}

func (s *OntologyObjectStore) ReplaceObjects(ontologyID string, objects []ontology.Object) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.objects == nil {
		s.objects = map[string][]ontology.Object{}
	}
	s.objects[strings.TrimSpace(ontologyID)] = append([]ontology.Object(nil), objects...)
	return nil
}

func (s *OntologyObjectStore) ListObjects(ontologyID string, filter ontology.ObjectFilter) ([]ontology.Object, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []ontology.Object{}
	for _, object := range s.objects[strings.TrimSpace(ontologyID)] {
		if !matchesAxis(filter.Subjects, object.SubjectID) ||
			!matchesAxis(filter.Modalities, object.Modality) ||
			!matchesAxis(filter.Visits, object.Visit) {
			continue
		}
		out = append(out, object)
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (s *OntologyObjectStore) CountObjects(ontologyID string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.objects[strings.TrimSpace(ontologyID)]), nil
}

// An empty axis is "no constraint", never "nothing".
func matchesAxis(wanted []string, value string) bool {
	if len(wanted) == 0 {
		return true
	}
	for _, candidate := range wanted {
		if candidate == value {
			return true
		}
	}
	return false
}
