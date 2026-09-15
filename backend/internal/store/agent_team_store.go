package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/agent"

// AgentTeamStore keeps the groups agents work in.
//
// Deliberately separate from AgentStore rather than folded into it. A team
// outlives the agents placed in it and is edited on its own - renamed, given a
// new purpose, emptied and refilled - and an interface that mixed the two
// would make every agent query carry a join nobody asked for.
type AgentTeamStore interface {
	ListByOwner(ownerUserID string) ([]agent.Team, error)
	GetByID(id string) (agent.Team, bool, error)
	Create(item agent.Team) error
	Update(item agent.Team) error
	// Delete removes the team. What happens to its members is the caller's
	// decision and is made explicitly: silently deleting agents here would
	// destroy standing work because somebody tidied up a grouping.
	Delete(id string) error
}
