package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/flow"

// FlowStore holds the graphs and their executions.
//
// The state of a run lives here rather than in a process: a scheduler holding
// the graph in memory would lose it at the next rollout, halfway through, with
// pods still running and nothing to reconcile them against.
type FlowStore interface {
	SaveFlow(item flow.Flow) error
	GetFlow(id string) (flow.Flow, bool, error)
	ListFlows(projectID string) ([]flow.Flow, error)
	DeleteFlow(id string) error

	CreateRun(run flow.Run, steps []flow.StepRun) error
	GetRun(id string) (flow.Run, bool, error)
	ListRuns(flowID string) ([]flow.Run, error)
	// ListActiveRuns is what the scheduler sweeps: the runs that are not over.
	ListActiveRuns() ([]flow.Run, error)
	UpdateRunStatus(runID, status string) error

	ListStepRuns(runID string) ([]flow.StepRun, error)
	UpdateStepRun(step flow.StepRun) error
}
