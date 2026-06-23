package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/backup"

type BackupRunStore interface {
	List() ([]backup.Run, error)
	GetByID(id string) (backup.Run, bool, error)
	Create(item backup.Run) error
	Update(item backup.Run) error
}
