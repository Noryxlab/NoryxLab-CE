package handlers

import "testing"

// A team is a subject, on the two things a regulated customer asks about.
//
// Teams reached projects and nothing else until 2026-09-26. The refusal was
// not a policy: the same line was written twice, once in datasets.go and once
// in ontology.go, and the account resolution never produced a team subject
// either - so a grant to a team could be neither written nor matched. Essilor
// found it by asking for one modality to be given to one of their teams and
// to one of Inria's.
func TestTeamIsAGrantableSubject(t *testing.T) {
	for _, subjectType := range []string{"user", "organization", "team"} {
		if !isGrantableSubjectType(subjectType) {
			t.Fatalf("%q must be grantable", subjectType)
		}
	}
}

// The list is closed. A subject type nobody resolves is a row that opens an
// asset to a group no screen will ever list.
func TestUnknownSubjectTypesStayRefused(t *testing.T) {
	for _, subjectType := range []string{"", "group", "project", "everyone", "Team", "TEAM"} {
		if isGrantableSubjectType(subjectType) {
			t.Fatalf("%q must not be grantable", subjectType)
		}
	}
}
