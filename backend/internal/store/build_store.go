package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/build"

type BuildStore interface {
	List() ([]build.Build, error)
	Create(b build.Build) error
}
