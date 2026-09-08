// Package cohort holds a named, frozen selection of files from an ontology.
//
// A cohort is the unit a study is actually run on: "the 23 subjects with a
// corneal wavefront, at their first visit". Two properties make it worth
// storing rather than recomputing.
//
// It is frozen. The selection is resolved to an explicit list of object paths
// when it is declared, so a cohort still names the same files after the study
// recruits eleven more subjects. A cohort that silently grew with its source
// would make last month's n unreproducible.
//
// It duplicates nothing. The paths point into the dataset where it already
// lives; mounting a cohort builds a tree of links, and not one byte is copied.
// The source stays read-only to the platform - these are regulated datasets,
// and the platform's job is to provide the tools, not to make second copies of
// the evidence.
package cohort

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Cohort struct {
	ID          string    `json:"id"`
	OntologyID  string    `json:"ontologyId"`
	ProjectID   string    `json:"projectId"`
	OwnerUserID string    `json:"ownerUserId"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Subjects    []string  `json:"subjects"`
	Modalities  []string  `json:"modalities"`
	Visits      []string  `json:"visits"`
	ObjectCount int       `json:"objectCount"`
	TotalBytes  int64     `json:"totalBytes"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Member is one file the cohort froze, with the place it takes in the tree a
// mount builds. The path is a key in the source bucket; nothing is written back
// to it.
type Member struct {
	CohortID  string `json:"cohortId"`
	Path      string `json:"path"`
	SubjectID string `json:"subjectId"`
	Visit     string `json:"visit"`
	Modality  string `json:"modality"`
	SizeBytes int64  `json:"sizeBytes"`
}

func New(ownerUserID, ontologyID, projectID, name, description string, subjects, modalities, visits []string) Cohort {
	now := time.Now().UTC()
	return Cohort{
		ID:          uuid.NewString(),
		OntologyID:  strings.TrimSpace(ontologyID),
		ProjectID:   strings.TrimSpace(projectID),
		OwnerUserID: strings.TrimSpace(ownerUserID),
		Name:        strings.TrimSpace(name),
		Description: strings.TrimSpace(description),
		Subjects:    normalize(subjects),
		Modalities:  normalize(modalities),
		Visits:      normalize(visits),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func normalize(values []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" || seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		out = append(out, trimmed)
	}
	return out
}
