package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/platformrole"

// PlatformRoleStore persists the rows an installation adds and the row each
// person holds. The shipped rows are not stored: they come from the code, and
// that is what keeps them from being edited.
type PlatformRoleStore interface {
	ListCustom() ([]platformrole.Role, error)
	GetCustom(key string) (platformrole.Role, bool, error)
	UpsertCustom(role platformrole.Role) error
	DeleteCustom(key string) error

	GetAssignment(userID string) (platformrole.Assignment, bool, error)
	SetAssignment(item platformrole.Assignment) error
	ClearAssignment(userID string) error
	// ListAssignments is every assignment at once, for the matrix's "held
	// by" column and the accounts screen.
	ListAssignments() ([]platformrole.Assignment, error)
}
