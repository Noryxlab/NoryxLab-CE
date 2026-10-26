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
