package memory

import (
	"encoding/json"
	"strings"
	"sync"
)

type ProjectOntologyStore struct {
	mu    sync.RWMutex
	items map[string]json.RawMessage
}

func NewProjectOntologyStore() *ProjectOntologyStore {
	return &ProjectOntologyStore{items: map[string]json.RawMessage{}}
}

func (s *ProjectOntologyStore) GetProjectOntology(projectID string) (json.RawMessage, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.items[strings.TrimSpace(projectID)]
	if !ok {
		return nil, false, nil
	}
	out := append(json.RawMessage(nil), value...)
	return out, true, nil
}

func (s *ProjectOntologyStore) UpsertProjectOntology(projectID, datasetID string, manifest json.RawMessage, generatedBy string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[strings.TrimSpace(projectID)] = append(json.RawMessage(nil), manifest...)
	return nil
}
