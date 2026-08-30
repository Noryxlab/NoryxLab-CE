package memory

import (
	"strings"
	"sync"
	"time"
)

type settingsEntry struct {
	value     string
	actor     string
	updatedAt time.Time
}

// SettingsStore keeps administrator overrides in memory, for installations
// running without a database.
type SettingsStore struct {
	mu    sync.RWMutex
	items map[string]settingsEntry
}

func NewSettingsStore() *SettingsStore {
	return &SettingsStore{items: map[string]settingsEntry{}}
}

func (s *SettingsStore) Get(key string) (string, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.items[strings.TrimSpace(key)]
	return entry.value, ok, nil
}

func (s *SettingsStore) Set(key, value, actor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	key = strings.TrimSpace(key)
	value = strings.TrimSpace(value)
	if value == "" {
		// Clearing a setting removes the override rather than storing an empty
		// string, so the environment value becomes visible again.
		delete(s.items, key)
		return nil
	}
	s.items[key] = settingsEntry{value: value, actor: actor, updatedAt: time.Now().UTC()}
	return nil
}

func (s *SettingsStore) List() (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(s.items))
	for key, entry := range s.items {
		out[key] = entry.value
	}
	return out, nil
}
