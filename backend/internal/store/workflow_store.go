package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workflow"

// WorkflowStore keeps workflow definitions and their runs.
//
// Runs are written after every step, which is the whole point: a run must be
// reconstructable from the store with no memory of the process that started
// it, so that a backend restart resumes work instead of losing it.
type WorkflowStore interface {
	ListByOwner(ownerUserID string) ([]workflow.Definition, error)
	// ListAll is for the scheduler, which runs on nobody's behalf.
	ListAll() ([]workflow.Definition, error)
	GetByID(id string) (workflow.Definition, bool, error)
	Create(item workflow.Definition) error
	Update(item workflow.Definition) error
	Delete(id string) error

	CreateRun(run workflow.Run) error
	UpdateRun(run workflow.Run) error
	GetRun(id string) (workflow.Run, bool, error)
	ListRuns(workflowID string, limit int) ([]workflow.Run, error)
	// ListUnfinishedRuns is what the scheduler resumes after a restart: every
	// run that is neither finished nor parked on a person.
	ListUnfinishedRuns() ([]workflow.Run, error)
}
