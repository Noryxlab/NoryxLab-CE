package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/storageendpoint"

type StorageEndpointStore interface {
	List() ([]storageendpoint.Endpoint, error)
	GetByID(id string) (storageendpoint.Endpoint, bool, error)
	Create(item storageendpoint.Endpoint) error
	Update(item storageendpoint.Endpoint) error
	Delete(id string) error
}
