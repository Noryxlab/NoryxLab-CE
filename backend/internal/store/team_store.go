package store

import (
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/team"
)

// ProjectTeamRole grants a role to every member of a team.
//
// The third way a role reaches a project, beside the person and the
// organization, and deliberately the same shape as both: resolving what
// somebody may do stays one comparison of three grants rather than a special
// case for the newest one.
type ProjectTeamRole struct {
	ProjectID string      `json:"projectId"`
	TeamID    string      `json:"teamId"`
	Role      access.Role `json:"role"`
	// TeamName is resolved for display. A grants table showing identifiers is
	// a table an administrator cannot audit, and the reader who most needs it
	// is the one who cannot list teams to find out.
	TeamName string `json:"teamName,omitempty"`
}

// TeamStore persists teams, who is in them, and what they may reach.
type TeamStore interface {
	ListByOrganization(organizationID string) ([]team.Team, error)
	GetByID(id string) (team.Team, bool, error)
	Create(item team.Team) error
	Update(item team.Team) error
	Delete(id string) error

	ListMembers(teamID string) ([]team.Member, error)
	AddMember(teamID, userID string) error
	RemoveMember(teamID, userID string) error
	// ListByUser is every team one person belongs to, which is what an
	// effective role has to consult on each request.
	ListByUser(userID string) ([]team.Team, error)

	// SetProjectRole grants a role to a team; an empty role revokes it.
	SetProjectRole(projectID, teamID string, role access.Role) error
	// ListProjectRoles returns the grants on one project.
	ListProjectRoles(projectID string) ([]ProjectTeamRole, error)
	// ListProjectRolesForUser returns the grants reaching one person through
	// any team they belong to.
	//
	// Asked as one query rather than by listing teams and then their grants,
	// because this runs on the authorization path of every request and two
	// round trips there is a cost paid by every screen.
	ListProjectRolesForUser(projectID, userID string) ([]ProjectTeamRole, error)
}
