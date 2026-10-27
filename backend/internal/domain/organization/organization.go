// Package organization holds what the platform decides about an organization,
// as opposed to what the directory knows about it.
//
// The directory answers one question: who works for whom. That is an identity
// fact, federated, and outside this platform's gift. It is not a permission,
// and the day it was read as one, every member of an organization held the
// strongest right there is over eight regulated buckets - read, write, delete
// and the power to hand them out - on the day their account was created.
//
// So the platform names the other half itself. An organization owns objects;
// somebody has to transfer them, rename them, delete them and decide who reads
// them. Those people are listed here, by name, and the list is the only thing
// that confers it. Belonging to an organization says where a person works;
// administering it is a right somebody granted.
package organization

import (
	"strings"
	"time"
)

// Admin is one subject allowed to administer what an organization owns.
//
// A person or a team, never an organization: an organization administering
// itself is the inheritance this whole object exists to end.
type Admin struct {
	OrganizationID string `json:"organizationId"`
	SubjectType    string `json:"subjectType"`
	SubjectID      string `json:"subjectId"`
	// SubjectName is resolved for display. A governance screen listing
	// identifiers is a screen nobody can audit, and this is the screen that
	// says who may give away health data.
	SubjectName string    `json:"subjectName,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Normalise lowercases the subject type and trims the identifiers, so that a
// comparison made on the authorization path never depends on how a screen
// spelled it.
func (a *Admin) Normalise() {
	a.OrganizationID = strings.TrimSpace(a.OrganizationID)
	a.SubjectType = strings.ToLower(strings.TrimSpace(a.SubjectType))
	a.SubjectID = strings.TrimSpace(a.SubjectID)
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now().UTC()
	}
}

// AdministrableSubjectType reports whether this subject may be named an
// administrator.
func AdministrableSubjectType(subjectType string) bool {
	switch strings.ToLower(strings.TrimSpace(subjectType)) {
	case "user", "team":
		return true
	}
	return false
}

// Policy is what belonging to this organization gives on what the
// organization owns: nothing, reading, or reading and writing.
//
// Belonging confers nothing by itself (ADR-048). What an organization gives
// its members is a grant - and this is the one place it is decided once
// rather than object by object: when an object is created for the
// organization or transferred to it, the platform writes the grant the
// policy names, to the organization as a subject, visible and revocable like
// any other. Changing the policy acts on the objects to come, never on the
// ones already held: lowering it must not withdraw in silence.
type Policy struct {
	OrganizationID  string    `json:"organizationId"`
	MembershipGrant string    `json:"membershipGrant"`
	UpdatedAt       time.Time `json:"updatedAt"`
	UpdatedBy       string    `json:"updatedBy,omitempty"`
}

const (
	MembershipGrantNone   = "none"
	MembershipGrantReader = "reader"
	MembershipGrantWriter = "writer"
)

// ValidMembershipGrant accepts the three answers and nothing else.
func ValidMembershipGrant(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case MembershipGrantNone, MembershipGrantReader, MembershipGrantWriter:
		return true
	}
	return false
}

func (p *Policy) Normalise() {
	p.OrganizationID = strings.TrimSpace(p.OrganizationID)
	p.MembershipGrant = strings.ToLower(strings.TrimSpace(p.MembershipGrant))
	if p.MembershipGrant == "" {
		p.MembershipGrant = MembershipGrantNone
	}
	if p.UpdatedAt.IsZero() {
		p.UpdatedAt = time.Now().UTC()
	}
}
