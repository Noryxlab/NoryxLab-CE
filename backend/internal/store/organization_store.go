package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/organization"

// OrganizationStore persists who administers what an organization owns.
//
// Deliberately small: the directory holds the organization, its name and its
// members, and none of that belongs here. What the platform owns is this one
// list, because it is a permission and a permission must not be federated
// away.
type OrganizationStore interface {
	ListAdmins(organizationID string) ([]organization.Admin, error)
	AddAdmin(item organization.Admin) error
	RemoveAdmin(organizationID, subjectType, subjectID string) error
	// AdministeredBy is every organization this caller administers, whether
	// named personally or through one of their teams.
	//
	// Asked as one query rather than by listing each organization's admins,
	// because it runs on the authorization path of every catalogue screen.
	AdministeredBy(userIDs []string, teamIDs []string) ([]string, error)
}
