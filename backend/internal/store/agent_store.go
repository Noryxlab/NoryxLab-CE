package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/agent"

type AgentStore interface {
	ListByOwner(ownerUserID string) ([]agent.Agent, error)
	// ListAll is for the scheduler, which runs on nobody's behalf and has to
	// see every agent to know which are due.
	ListAll() ([]agent.Agent, error)
	GetByID(id string) (agent.Agent, bool, error)
	Create(item agent.Agent) error
	Update(item agent.Agent) error
	Delete(id string) error

	AppendRun(run agent.Run) error
	UpdateRun(run agent.Run) error
	ListRuns(agentID string, limit int) ([]agent.Run, error)
}
