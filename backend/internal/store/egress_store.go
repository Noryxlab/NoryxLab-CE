package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/egress"

type EgressRuleStore interface {
	List() ([]egress.Rule, error)
	ListByProject(projectID string) ([]egress.Rule, error)
	GetByID(id string) (egress.Rule, bool, error)
	Create(item egress.Rule) error
	UpdateDecision(id, status, reviewerID, note string) error
}
