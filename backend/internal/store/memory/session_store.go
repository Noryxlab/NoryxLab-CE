package memory

import (
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/session"
)

type SessionStore struct {
	mu    sync.RWMutex
	items map[string]session.Session
}

func NewSessionStore() *SessionStore {
	return &SessionStore{items: map[string]session.Session{}}
}

func (s *SessionStore) Create(item session.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.items[item.Token] = item
	return nil
}

func (s *SessionStore) Get(token string) (session.Session, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	item, ok := s.items[token]
	return item, ok, nil
}

func (s *SessionStore) Delete(token string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.items, token)
	return nil
}
