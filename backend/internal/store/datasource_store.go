package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/datasource"

type DatasourceStore interface {
	ListByUser(userID string) ([]datasource.Datasource, error)
	GetByID(id string) (datasource.Datasource, bool, error)
	Create(item datasource.Datasource) error
	Delete(id string) error
}
