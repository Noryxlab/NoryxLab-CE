// Package team holds a named group of people inside an organization.
//
// It fills the gap between the two things the platform could already name. An
// organization is an identity fact - who someone works for - and it lives in
// the directory, where it is federated and outside the platform's gift. A
// person is an individual grant, made one project at a time. Neither describes
// the unit work is actually organised in: five people on the same study, a
// squad, a client account.
//
// Without it, an administrator equipping a new researcher has to remember
// every project that person should reach, and the grants drift apart quietly
// because nothing ever says they were meant to match. A team is that intent,
// written down once.
//
// It is the platform's object and not the directory's, deliberately. A team
// carries consumption, quotas and reporting - all questions about what a group
// costs and uses, none of which an identity provider answers. The day an
// installation federates its directory, its AD groups must not silently become
// the unit this platform bills against.
//
// Teams nest under an organization rather than floating: a team without an
// organization has no owner to answer for it, and the grant it carries would
// outlive every arrangement that justified it.
package team

import (
	"strings"
	"time"
)

// Team is a group of people within one organization.
type Team struct {
	ID string `json:"id"`
	// OrganizationID is the directory's identifier for the organization this
	// team belongs to. Stored as given: the platform does not own it, and
	// resolving it to a name is the reader's problem, not the record's.
	OrganizationID string `json:"organizationId"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	// OrganizationName is resolved for display, like a project's owner name.
	// A screen that shows 57c801a4-f273-4844-97c6-28307872a480 tells its
	// reader nothing, and a non-administrator cannot list organizations to
	// find out.
	OrganizationName string `json:"organizationName,omitempty"`
	// MemberCount is filled by listings so a screen does not have to ask once
	// per row. Absent from a single read, where the members themselves are
	// what the caller wanted.
	MemberCount int       `json:"memberCount,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Member is one person's place in a team.
//
// The moment of joining is kept because a grant that reaches somebody through
// a team is answerable like any other: an auditor asking why this person could
// read that dataset in March needs to know they were in the team in March.
type Member struct {
	TeamID   string    `json:"teamId"`
	UserID   string    `json:"userId"`
	JoinedAt time.Time `json:"joinedAt"`
}

// MaxNameLength bounds what a caller may store.
//
// A name is shown in tables, in audit lines and in reports, and an unbounded
// one is a caller's free choice of how wide every screen becomes.
const MaxNameLength = 120

// MaxDescriptionLength bounds the free text for the same reason, with more
// room because a description is read once rather than listed.
const MaxDescriptionLength = 1000

// Normalise trims what a caller supplied and reports whether the result can be
// stored.
//
// Validation lives with the type rather than in each handler, so the API, an
// import and a future command-line all apply the same rule. A name that is
// only whitespace is not a name: accepting it produces a row nobody can refer
// to and an interface with an empty cell.
func (t *Team) Normalise() error {
	t.Name = strings.TrimSpace(t.Name)
	t.Description = strings.TrimSpace(t.Description)
	t.OrganizationID = strings.TrimSpace(t.OrganizationID)

	switch {
	case t.Name == "":
		return ErrNameRequired
	case len([]rune(t.Name)) > MaxNameLength:
		return ErrNameTooLong
	case len([]rune(t.Description)) > MaxDescriptionLength:
		return ErrDescriptionTooLong
	case t.OrganizationID == "":
		return ErrOrganizationRequired
	}
	return nil
}
