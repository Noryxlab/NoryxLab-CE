package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/cohort"

// CohortStore persists a named selection of files and the list it froze.
type CohortStore interface {
	ListByProject(projectID string) ([]cohort.Cohort, error)
	ListByOntology(ontologyID string) ([]cohort.Cohort, error)
	GetByID(id string) (cohort.Cohort, bool, error)
	Create(item cohort.Cohort, members []cohort.Member) error
	ListMembers(cohortID string, limit int) ([]cohort.Member, error)
	Delete(id string) error
}
