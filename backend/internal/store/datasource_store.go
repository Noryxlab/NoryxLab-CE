package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/datasource"

type DatasourceStore interface {
	ListByUser(userID string) ([]datasource.Datasource, error)
	ListAll() ([]datasource.Datasource, error)
	GetByID(id string) (datasource.Datasource, bool, error)
	Create(item datasource.Datasource) error
	Upsert(item datasource.Datasource) error
	// SetOwner hands the connector to somebody else, and touches nothing else.
	SetOwner(id, ownerType, ownerID string) error
	Delete(id string) error
}
