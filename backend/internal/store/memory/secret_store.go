package memory

import (
	"strings"
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/secret"
)

type SecretStore struct {
	mu    sync.RWMutex
	items []secret.Secret
}

func NewSecretStore() *SecretStore {
	return &SecretStore{items: []secret.Secret{}}
}

func (s *SecretStore) ListByUser(userID string) ([]secret.Secret, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	userID = strings.TrimSpace(userID)
	out := make([]secret.Secret, 0)
	for _, item := range s.items {
		if item.UserID == userID {
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *SecretStore) GetByName(userID, name string) (secret.Secret, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	userID = strings.TrimSpace(userID)
	name = strings.TrimSpace(name)
	for _, item := range s.items {
		if item.UserID == userID && item.Name == name {
			return item, true, nil
		}
	}
	return secret.Secret{}, false, nil
}

func (s *SecretStore) Upsert(in secret.Secret) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, item := range s.items {
		if item.UserID == in.UserID && item.Name == in.Name {
			s.items[i] = in
			return nil
		}
	}
	s.items = append(s.items, in)
	return nil
}

func (s *SecretStore) Delete(userID, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, item := range s.items {
		if item.UserID == strings.TrimSpace(userID) && item.Name == strings.TrimSpace(name) {
			s.items = append(s.items[:i], s.items[i+1:]...)
			break
		}
	}
	return nil
}
