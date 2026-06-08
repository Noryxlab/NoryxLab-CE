package memory

import (
	"strings"
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
)

type DatasetStore struct {
	mu     sync.RWMutex
	items  []dataset.Dataset
	access []dataset.Access
}

func NewDatasetStore() *DatasetStore {
	return &DatasetStore{items: []dataset.Dataset{}, access: []dataset.Access{}}
}

func (s *DatasetStore) ListBySubjects(subjects []dataset.Subject) ([]dataset.Dataset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]dataset.Dataset, 0)
	for _, item := range s.items {
		for _, subject := range subjects {
			if item.OwnerType == subject.Type && item.OwnerID == strings.TrimSpace(subject.ID) {
				item.AccessRole = "owner"
				out = append(out, item)
				break
			}
		}
		if item.AccessRole == "owner" {
			continue
		}
		best := ""
		for _, access := range s.access {
			for _, subject := range subjects {
				if access.DatasetID == item.ID && access.SubjectType == subject.Type && access.SubjectID == strings.TrimSpace(subject.ID) {
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

func (s *DatasetStore) UpdateOwner(datasetID, ownerType, ownerID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == strings.TrimSpace(datasetID) {
			s.items[i].OwnerType = strings.TrimSpace(ownerType)
			s.items[i].OwnerID = strings.TrimSpace(ownerID)
			if ownerType == "user" {
				s.items[i].OwnerUserID = strings.TrimSpace(ownerID)
			}
		}
	}
	return nil
}

func (s *DatasetStore) ListAll() ([]dataset.Dataset, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]dataset.Dataset(nil), s.items...), nil
}

func (s *DatasetStore) GetByID(id string) (dataset.Dataset, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.items {
		if item.ID == strings.TrimSpace(id) {
			return item, true, nil
		}
	}
	return dataset.Dataset{}, false, nil
}

func (s *DatasetStore) Create(item dataset.Dataset) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
	return nil
}

func (s *DatasetStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	target := strings.TrimSpace(id)
	filtered := make([]dataset.Dataset, 0, len(s.items))
	for _, item := range s.items {
		if item.ID != target {
			filtered = append(filtered, item)
		}
	}
	s.items = filtered
	access := s.access[:0]
	for _, item := range s.access {
		if item.DatasetID != target {
			access = append(access, item)
		}
	}
	s.access = access
	return nil
}

func (s *DatasetStore) ListAccess(datasetID string) ([]dataset.Access, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []dataset.Access{}
	for _, item := range s.access {
		if item.DatasetID == strings.TrimSpace(datasetID) {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *DatasetStore) GetAccess(datasetID, subjectType, subjectID string) (dataset.Access, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, item := range s.access {
		if item.DatasetID == strings.TrimSpace(datasetID) && item.SubjectType == strings.TrimSpace(subjectType) && item.SubjectID == strings.TrimSpace(subjectID) {
			return item, true, nil
		}
	}
	return dataset.Access{}, false, nil
}

func (s *DatasetStore) SetAccess(item dataset.Access) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.access {
		if s.access[i].DatasetID == item.DatasetID && s.access[i].SubjectType == item.SubjectType && s.access[i].SubjectID == item.SubjectID {
			s.access[i] = item
			return nil
		}
	}
	s.access = append(s.access, item)
	return nil
}

func (s *DatasetStore) DeleteAccess(datasetID, subjectType, subjectID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.access[:0]
	for _, item := range s.access {
		if item.DatasetID != strings.TrimSpace(datasetID) || item.SubjectType != strings.TrimSpace(subjectType) || item.SubjectID != strings.TrimSpace(subjectID) {
			out = append(out, item)
		}
	}
	s.access = out
	return nil
}
