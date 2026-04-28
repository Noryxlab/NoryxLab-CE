package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/job"

type JobStore interface {
	List() ([]job.Job, error)
	GetByID(id string) (job.Job, bool, error)
	Create(item job.Job) error
	Upsert(item job.Job) error
	Delete(id string) error
}
