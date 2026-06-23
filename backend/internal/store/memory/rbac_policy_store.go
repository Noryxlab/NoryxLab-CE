package memory

import (
	"encoding/json"
	"sync"
)

type RBACPolicyStore struct {
	mu     sync.RWMutex
	policy json.RawMessage
}

func NewRBACPolicyStore() *RBACPolicyStore {
	return &RBACPolicyStore{}
}

func (s *RBACPolicyStore) Get() (json.RawMessage, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.policy) == 0 {
		return nil, false, nil
	}
	out := append(json.RawMessage(nil), s.policy...)
	return out, true, nil
}

func (s *RBACPolicyStore) Set(policy json.RawMessage) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.policy = append(json.RawMessage(nil), policy...)
	return nil
}
