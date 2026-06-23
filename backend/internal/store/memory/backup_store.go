package memory

import (
	"strings"
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/backup"
)

type BackupRunStore struct {
	mu    sync.RWMutex
	items []backup.Run
}

func NewBackupRunStore() *BackupRunStore {
	return &BackupRunStore{items: []backup.Run{}}
}

func (s *BackupRunStore) List() ([]backup.Run, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]backup.Run, len(s.items))
	copy(out, s.items)
	return out, nil
}

func (s *BackupRunStore) GetByID(id string) (backup.Run, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	id = strings.TrimSpace(id)
	for _, item := range s.items {
		if item.ID == id {
			return item, true, nil
		}
	}
	return backup.Run{}, false, nil
}

func (s *BackupRunStore) Create(item backup.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items = append(s.items, item)
	return nil
}

func (s *BackupRunStore) Update(item backup.Run) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.items {
		if s.items[i].ID == item.ID {
			s.items[i] = item
			return nil
		}
	}
	s.items = append(s.items, item)
	return nil
}
