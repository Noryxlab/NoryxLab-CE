package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/platformrole"

// PlatformRoleStore persists the rows an installation adds, what it changed
// on the shipped ones, and the row each person holds. A shipped row itself is
// never stored: it comes from the code, and only the difference an
// installation chose is kept beside it.
type PlatformRoleStore interface {
	ListCustom() ([]platformrole.Role, error)
	GetCustom(key string) (platformrole.Role, bool, error)
	UpsertCustom(role platformrole.Role) error
	DeleteCustom(key string) error

	// ListOverrides is what this installation changed on the shipped rows,
	// kept as a difference so a shipped row that gains a function still
	// gains it here.
	ListOverrides() ([]platformrole.Override, error)
	SetOverride(item platformrole.Override) error

	GetAssignment(userID string) (platformrole.Assignment, bool, error)
	SetAssignment(item platformrole.Assignment) error
	ClearAssignment(userID string) error
	// ListAssignments is every assignment at once, for the matrix's "held
	// by" column and the accounts screen.
	ListAssignments() ([]platformrole.Assignment, error)
}
