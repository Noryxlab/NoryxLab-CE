// Package extract holds a named, frozen selection of files from an ontology.
//
// An extract is the unit a study is actually run on: "the 23 subjects with a
// corneal wavefront, at their first visit". Two properties make it worth
// storing rather than recomputing.
//
// It is frozen. The selection is resolved to an explicit list of object paths
// when it is declared, so an extract still names the same files after the study
// recruits eleven more subjects. An extract that silently grew with its source
// would make last month's n unreproducible.
//
// It duplicates nothing. The paths point into the dataset where it already
// lives; mounting an extract builds a tree of links, and not one byte is copied.
// The source stays read-only to the platform - these are regulated datasets,
// and the platform's job is to provide the tools, not to make second copies of
// the evidence.
package extract

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Extract struct {
	ID          string `json:"id"`
	OntologyID  string `json:"ontologyId"`
	ProjectID   string `json:"projectId"`
	OwnerUserID string `json:"ownerUserId"`
	// OwnerType and OwnerID carry the same meaning as on a dataset or an
	// ontology: who the extract belongs to, and therefore who answers for it.
	//
	// It had only OwnerUserID, so it could not be handed to an organization -
	// a frozen selection of health data belonged to whoever happened to
	// declare it, and left with them. The catalogue's other objects have been
	// transferable for a while; this one simply had not caught up.
	OwnerType string `json:"ownerType"`
	OwnerID   string `json:"ownerId"`
	// OwnerName is what a screen shows, resolved from the directory for an
	// organization and equal to the username for a person.
	OwnerName   string   `json:"ownerName,omitempty"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Subjects    []string `json:"subjects"`
	Modalities  []string `json:"modalities"`
	Visits      []string `json:"visits"`
	// Formats is the kinds of file this extract keeps. Recorded with the
	// selection, because an extract that says "ANTERION" and holds only its
	// CSVs is a different thing from one that holds its images, and the
	// difference has to be legible a year later.
	Formats []string `json:"formats"`
	// Layout is the order of the directory levels a mount builds, and it is
	// the other half of what an extract is.
	//
	// A selection says *which* files; a layout says how they are arranged to
	// work on. The same selection organised subject-first answers "what does
	// this patient have", and modality-first answers "show me every cornea
	// scan I hold" - two different questions, one set of files, and until now
	// only the first was expressible because the tree was built in a fixed
	// order.
	//
	// A permutation of the three levels, never a subset: dropping one would
	// put files from different visits in the same directory, where the ones
	// sharing a name would overwrite each other and the study would quietly
	// lose rows. Empty means the default, which is what every extract
	// declared before this carried.
	Layout      []string  `json:"layout,omitempty"`
	ObjectCount int       `json:"objectCount"`
	TotalBytes  int64     `json:"totalBytes"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Member is one file the extract froze, with the place it takes in the tree a
// mount builds. The path is a key in the source bucket; nothing is written back
// to it.
type Member struct {
	ExtractID string `json:"extractId"`
	Path      string `json:"path"`
	SubjectID string `json:"subjectId"`
	Visit     string `json:"visit"`
	Modality  string `json:"modality"`
	SizeBytes int64  `json:"sizeBytes"`
}

func New(ownerUserID, ontologyID, projectID, name, description string, subjects, modalities, visits, formats []string) Extract {
	now := time.Now().UTC()
	return Extract{
		ID:          uuid.NewString(),
		OntologyID:  strings.TrimSpace(ontologyID),
		ProjectID:   strings.TrimSpace(projectID),
		OwnerUserID: strings.TrimSpace(ownerUserID),
		// Le meme defaut que partout ailleurs : son auteur, jusqu a ce que
		// quelqu un le transfere.
		OwnerType:   "user",
		OwnerID:     strings.TrimSpace(ownerUserID),
		Name:        strings.TrimSpace(name),
		Description: strings.TrimSpace(description),
		Subjects:    normalize(subjects),
		Modalities:  normalize(modalities),
		Visits:      normalize(visits),
		Formats:     normalize(formats),
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

// LevelSubject, LevelVisit and LevelModality are the three levels a mount can
// be organised by. They are the ontology's own vocabulary - the one the scan
// inferred and the one the filters use - so a layout names nothing new.
const (
	LevelSubject  = "subject"
	LevelVisit    = "visit"
	LevelModality = "modality"
)

// DefaultLayout is subject first: the arrangement every extract had before a
// layout could be chosen, kept as the default so nothing moves underneath an
// extract declared earlier.
func DefaultLayout() []string {
	return []string{LevelSubject, LevelVisit, LevelModality}
}

// NormaliseLayout accepts one, two or three distinct levels, in any order.
//
// All three used to be required, so the tree always carried every level even
// when one of them said nothing: on a study with a single visit per patient,
// the middle directory holds one entry per patient and exists only to be
// walked through. Dropping it is the arrangement somebody actually wants.
//
// Any order, too. The caller used to be offered three permutations of six,
// for no reason beyond the three the screen happened to list - and "all my
// cornea scans, whoever they belong to" is a different question from "what
// does this patient have", over the same files.
//
// Empty is the default rather than an error: an extract declared before
// layouts existed has none, and a caller that does not care should not have to
// name one. Anything else is refused with a reason, because a layout that was
// silently corrected would arrange somebody's study differently from what they
// asked for and say nothing.
//
// A shorter layout can make two files land in the same place, and this
// function cannot know: it sees a rule, not the files. The caller checks that
// against the selection before freezing it - see Collision.
func NormaliseLayout(raw []string) ([]string, string) {
	if len(raw) == 0 {
		return DefaultLayout(), ""
	}
	seen := map[string]bool{}
	out := make([]string, 0, 3)
	for _, level := range raw {
		level = strings.ToLower(strings.TrimSpace(level))
		switch level {
		case LevelSubject, LevelVisit, LevelModality:
		default:
			return nil, "a layout level must be one of subject, visit or modality"
		}
		if seen[level] {
			return nil, "a layout names each level once"
		}
		seen[level] = true
		out = append(out, level)
	}
	return out, ""
}

// Omitted names the levels a layout leaves out, in the platform's own order.
//
// Used to say what to add back when a layout files two files in one place -
// "add the visit" is actionable where "this layout collides" is not.
func Omitted(layout []string) []string {
	present := map[string]bool{}
	for _, level := range layout {
		present[level] = true
	}
	out := []string{}
	for _, level := range []string{LevelSubject, LevelVisit, LevelModality} {
		if !present[level] {
			out = append(out, level)
		}
	}
	return out
}

// DirectoryFor places one member in the tree, following the layout.
func DirectoryFor(layout []string, subject, visit, modality string) []string {
	if len(layout) == 0 {
		layout = DefaultLayout()
	}
	out := make([]string, 0, len(layout))
	for _, level := range layout {
		switch level {
		case LevelSubject:
			out = append(out, subject)
		case LevelVisit:
			out = append(out, visit)
		case LevelModality:
			out = append(out, modality)
		}
	}
	return out
}
