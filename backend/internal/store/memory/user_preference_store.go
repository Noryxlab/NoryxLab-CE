package memory

import (
	"strings"
	"sync"
)

type UserPreferenceStore struct {
	mu    sync.RWMutex
	items map[string]map[string]string
}

func NewUserPreferenceStore() *UserPreferenceStore {
	return &UserPreferenceStore{
		items: map[string]map[string]string{},
	}
}

func (s *UserPreferenceStore) Get(userID, key string) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	userID = strings.TrimSpace(userID)
	key = strings.TrimSpace(key)
	if userID == "" || key == "" {
		return "", false, nil
	}
	row, ok := s.items[userID]
	if !ok {
		return "", false, nil
	}
	value, ok := row[key]
	if !ok {
		return "", false, nil
	}
	return value, true, nil
}

func (s *UserPreferenceStore) Set(userID, key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	userID = strings.TrimSpace(userID)
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if userID == "" || key == "" {
		return nil
	}
	if _, ok := s.items[userID]; !ok {
		s.items[userID] = map[string]string{}
	}
	s.items[userID][key] = value
	return nil
}

