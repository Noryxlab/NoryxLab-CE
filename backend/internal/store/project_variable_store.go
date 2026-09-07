package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/projectvar"

type ProjectVariableStore interface {
	ListByProject(projectID string) ([]projectvar.Variable, error)
	// ListAll returns every variable across projects. For platform-wide
	// operations such as backup; never expose it on a user-facing endpoint.
	ListAll() ([]projectvar.Variable, error)
	GetByName(projectID, name string) (projectvar.Variable, bool, error)
	Upsert(item projectvar.Variable) error
	Delete(projectID, name string) error
}
