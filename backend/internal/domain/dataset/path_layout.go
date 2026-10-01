package dataset

import (
	"fmt"
	"strconv"
	"strings"
)

// How a dataset's object paths are read.
//
// The platform read every bucket with one hardcoded rule: find a segment that
// looks like a subject identifier, then take the next two as the visit and the
// modality. It worked because the first two studies to arrive were filed that
// way, and the day a third one numbers its patients differently - SELENA-01-001
// against PREMYOM1000-0001, or a centre code before the patient - the scan
// recognises nothing and somebody has to write Go to fix it.
//
// A layout is that rule, written down per dataset instead of compiled in. It is
// positional on purpose:
//
//   - it is readable by the person who owns the data - "level 2 is the subject"
//     is a sentence a clinician can check against one path and confirm;
//   - it is writable by hand, in three fields, so an installation with no
//     assistant configured is not blocked by its absence;
//   - and it is exactly what the assistant already produces in prose when it is
//     shown path shapes, so "apply what it proposed" is a transcription rather
//     than an interpretation.
//
// What it deliberately is not is a regular expression. A rule somebody can get
// subtly wrong, over health data, filed under the wrong patient and discovered
// months later, is not a feature - it is a hazard with a text field.
type PathLayout struct {
	// SubjectLevel is which path segment holds the subject identifier,
	// counted from zero. Required: a layout that names no subject describes
	// nothing the platform can group by.
	SubjectLevel int `json:"subjectLevel"`
	// VisitLevel and ModalityLevel are optional. A study with one visit per
	// patient, or one instrument, simply has no segment for it - and saying
	// "absent" is truer than pointing at a segment that means something else.
	VisitLevel    int `json:"visitLevel"`
	ModalityLevel int `json:"modalityLevel"`
}

// LevelAbsent marks a level this layout does not name.
const LevelAbsent = -1

// Declared reports whether a dataset carries a layout at all. The zero value
// does not: level 0 for all three would be a rule, and an accidental one.
func (l PathLayout) Declared() bool {
	return l.SubjectLevel >= 0
}

// Describe writes the layout the way a screen should show it, which is also
// the way somebody checks it against a path they know.
func (l PathLayout) Describe() string {
	if !l.Declared() {
		return "no layout declared"
	}
	parts := []string{fmt.Sprintf("level %d = subject", l.SubjectLevel)}
	if l.VisitLevel >= 0 {
		parts = append(parts, fmt.Sprintf("level %d = visit", l.VisitLevel))
	}
	if l.ModalityLevel >= 0 {
		parts = append(parts, fmt.Sprintf("level %d = modality", l.ModalityLevel))
	}
	return strings.Join(parts, ", ")
}

// Validate refuses a rule rather than correcting it.
//
// A layout that was silently adjusted would file somebody's data under a
// segment they did not choose, and the mistake would surface as a subject
// count nobody can explain.
func (l PathLayout) Validate() string {
	if l.SubjectLevel < 0 {
		return "a layout must say which level holds the subject"
	}
	if l.SubjectLevel > 15 || l.VisitLevel > 15 || l.ModalityLevel > 15 {
		return "a path level above 15 is not a layout, it is a typo"
	}
	if l.VisitLevel >= 0 && l.VisitLevel == l.SubjectLevel {
		return "the visit and the subject cannot be the same level"
	}
	if l.ModalityLevel >= 0 && l.ModalityLevel == l.SubjectLevel {
		return "the modality and the subject cannot be the same level"
	}
	if l.VisitLevel >= 0 && l.ModalityLevel >= 0 && l.VisitLevel == l.ModalityLevel {
		return "the visit and the modality cannot be the same level"
	}
	return ""
}

// Read applies the layout to one object path.
//
// A path too short for the rule yields no subject, which is what makes the
// scan count it as unrecognised and describe its shape rather than guess. That
// is the same answer the hardcoded rule gave, and for the same reason: a file
// the rule does not cover is a fact to report, never a file to file somewhere
// plausible.
func (l PathLayout) Read(relPath string) (subject, visit, modality string) {
	segments := strings.Split(strings.Trim(relPath, "/"), "/")
	at := func(level int) string {
		if level < 0 || level >= len(segments) {
			return ""
		}
		return strings.TrimSpace(segments[level])
	}
	subject = at(l.SubjectLevel)
	if subject == "" {
		return "", "", ""
	}
	// The last segment is the file itself. A rule pointing at it would make
	// every file its own subject, which is the kind of mistake that produces
	// "4,026 subjects" and reads as a broken scan.
	if l.SubjectLevel == len(segments)-1 {
		return "", "", ""
	}
	visit = strings.TrimPrefix(at(l.VisitLevel), "visit_")
	modality = strings.TrimPrefix(at(l.ModalityLevel), "modality_")
	if modality != "" {
		modality = strings.ToUpper(modality)
	}
	return subject, visit, modality
}

// ParseLayoutLevel reads a level from a form field, where empty means absent.
func ParseLayoutLevel(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return LevelAbsent, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return LevelAbsent, fmt.Errorf("a path level is a whole number, counted from zero")
	}
	return value, nil
}
