package store

import (
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/budget"
)

// BudgetStore holds what a project may consume, and answers what it has.
//
// Consumption is not a field here: it is derived from the usage samples, so
// there is never a second total to keep in step with the first.
type BudgetStore interface {
	SetProjectBudget(item budget.Budget) error
	GetProjectBudget(projectID string) (budget.Budget, bool, error)
	DeleteProjectBudget(projectID string) error
	// ListBudgetStates joins allowance and consumption for one period,
	// including projects with no budget - seeing consumption is what lets
	// somebody decide what a budget should be.
	ListBudgetStates(period time.Time) ([]budget.State, error)
	// MarkBudgetAnnounced records that a threshold was announced, and says
	// whether this call is the one that announced it. Atomic on purpose: the
	// watcher passes every few minutes, and a check-then-write would repeat
	// the same warning for a month.
	MarkBudgetAnnounced(projectID string, period time.Time, threshold int) (bool, error)
}
