package memory

import (
	"sort"
	"sync"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/apitoken"
)

type APITokenStore struct {
	mu   sync.RWMutex
	byID map[string]apitoken.Token
}

func NewAPITokenStore() *APITokenStore {
	return &APITokenStore{byID: map[string]apitoken.Token{}}
}

func (s *APITokenStore) Put(token apitoken.Token) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[token.ID] = token
	return nil
}

func (s *APITokenStore) Get(id string) (apitoken.Token, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	token, ok := s.byID[id]
	return token, ok, nil
}

func (s *APITokenStore) ListByUser(userID string) ([]apitoken.Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := []apitoken.Token{}
	for _, token := range s.byID {
		if token.UserID == userID {
			out = append(out, token)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

// Revoke takes the owner as well as the identifier, so a caller cannot revoke
// somebody else's token by guessing an id.
func (s *APITokenStore) Revoke(id, userID string, at time.Time) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, ok := s.byID[id]
	if !ok || token.UserID != userID {
		return false, nil
	}
	if token.RevokedAt == nil {
		stamp := at.UTC()
		token.RevokedAt = &stamp
		s.byID[id] = token
	}
	return true, nil
}

func (s *APITokenStore) Touch(id string, at time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	token, ok := s.byID[id]
	if !ok {
		return nil
	}
	stamp := at.UTC()
	token.LastUsedAt = &stamp
	s.byID[id] = token
	return nil
}
