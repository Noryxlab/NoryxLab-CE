package team

import "errors"

// The refusals a caller can act on.
//
// Named rather than formatted at the point of failure so a handler can map
// each one onto its status code without matching on message text - a habit
// that breaks the first time somebody improves the wording.
var (
	ErrNameRequired         = errors.New("a team needs a name")
	ErrNameTooLong          = errors.New("the team name is too long")
	ErrDescriptionTooLong   = errors.New("the team description is too long")
	ErrOrganizationRequired = errors.New("a team belongs to an organization")
	ErrNotFound             = errors.New("no such team")
	// ErrNameTaken is returned when an organization already has a team by
	// that name. Unique within the organization and not globally: two
	// customers may each have a "Data Science", and forcing them to differ
	// would leak one installation's tenants to another.
	ErrNameTaken = errors.New("this organization already has a team with that name")
)
