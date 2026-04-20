package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"

type ProjectStore interface {
    List() ([]project.Project, error)
    Create(p project.Project) error
}
