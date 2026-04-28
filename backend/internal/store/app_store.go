package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/app"

type AppStore interface {
	List() ([]app.App, error)
	GetByID(id string) (app.App, bool, error)
	GetBySlug(slug string) (app.App, bool, error)
	Create(item app.App) error
	Upsert(item app.App) error
	Delete(id string) error
}
