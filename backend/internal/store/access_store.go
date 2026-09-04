package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"

type ProjectRole struct {
	ProjectID string      `json:"projectId"`
	UserID    string      `json:"userId"`
	Role      access.Role `json:"role"`
}

// ProjectOrganizationRole grants a role to every member of an organization.
//
// Assigning people one at a time is workable for a team of five and a reason
// not to buy at fifty: an administrator adding a researcher has to remember
// every project that person should reach. A grant to the organization is made
// once and follows its membership.
type ProjectOrganizationRole struct {
	ProjectID      string      `json:"projectId"`
	OrganizationID string      `json:"organizationId"`
	Role           access.Role `json:"role"`
}

type AccessStore interface {
	SetRole(projectID, userID string, role access.Role)
	GetRole(projectID, userID string) (access.Role, bool)
	ListProjectRoles() ([]ProjectRole, error)

	// SetOrganizationRole grants a role to an organization; an empty role
	// revokes it.
	SetOrganizationRole(projectID, organizationID string, role access.Role) error
	// ListOrganizationRoles returns the grants on one project.
	ListOrganizationRoles(projectID string) ([]ProjectOrganizationRole, error)
}
