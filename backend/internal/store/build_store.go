package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/build"

type BuildStore interface {
	List() ([]build.Build, error)
	GetByID(id string) (build.Build, bool, error)
	Create(b build.Build) error
	Upsert(b build.Build) error
}
