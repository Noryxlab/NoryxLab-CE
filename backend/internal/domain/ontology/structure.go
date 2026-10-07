package ontology

import (
	"sort"
	"strings"
	"time"
)

// Reading a file's technical description, and nothing else (ADR-047).
//
// The path scan reads keys. This reads *inside* files, which is a different
// act and needs saying out loud: it opens regulated data. Authorised for
// hds-for on 2026-10-07, and the conditions are not what the authorisation
// relaxed - they are what it rests on.
//
// The whole design is one distinction, applied per field rather than per file:
//
//	structure  the list of tags a DICOM carries, Modality, Rows, PixelSpacing
//	content    PatientName, PatientID, free text, pixels
//
// "These 3 592 DICOMs carry these 42 tags, and 3 588 declare Modality = OP" is
// structure. "PatientName is X" is content, and also an identifier. The line is
// per field, which is why it has to be a list rather than a principle - and why
// the list is data on the dataset rather than a constant in this binary,
// following ADR-040's reasoning about the reading rule.

// StructureAllowlist is what a scan may read, for one format.
//
// Two separate lists, because they are read for opposite reasons. Record is
// read in order to be kept; Identifying is read in order to be *checked* and
// never kept - the difference between describing a dataset and copying it.
type StructureAllowlist struct {
	// Format is the file family this applies to: "dicom", "nifti", "parquet".
	Format string `json:"format"`
	// Record are the fields whose values may be summarised. Low cardinality
	// and non-identifying, both, because a distribution over a
	// high-cardinality field is a copy of the column with extra steps.
	Record []string `json:"record,omitempty"`
	// Identifying are the fields that must carry no value in a pseudonymised
	// dataset. A scan looks at them, reports whether anything was there, and
	// records no value - not even a hash, which is still a per-subject key.
	Identifying []string `json:"identifying,omitempty"`
}

// Declared reports whether this list says anything at all.
func (a StructureAllowlist) Declared() bool {
	return strings.TrimSpace(a.Format) != "" && (len(a.Record) > 0 || len(a.Identifying) > 0)
}

// Problem reports why an allowlist would be refused, or "".
//
// A field in both lists is the mistake this exists to catch: it would be read
// to be checked and read to be kept, and the second wins by accident. On a
// regulated bucket that is how an identifier ends up in a card.
func (a StructureAllowlist) Problem() string {
	if strings.TrimSpace(a.Format) == "" {
		return "a structure allowlist needs a format"
	}
	if len(a.Record) == 0 && len(a.Identifying) == 0 {
		return "a structure allowlist that names no field reads nothing"
	}
	identifying := map[string]bool{}
	for _, field := range a.Identifying {
		identifying[normaliseField(field)] = true
	}
	for _, field := range a.Record {
		if identifying[normaliseField(field)] {
			return "field " + strings.TrimSpace(field) +
				" is listed as both recorded and identifying; a field read to be checked must not be read to be kept"
		}
	}
	return ""
}

func normaliseField(field string) string {
	return strings.ToLower(strings.TrimSpace(field))
}

// FieldTally is how often a value appeared, for one recorded field.
//
// A count per value and no object keys, which is the "distributions only" rule
// in its concrete form: this says what the dataset looks like, and cannot be
// read backwards to say what any one file contains.
type FieldTally struct {
	Field string `json:"field"`
	// Values maps a value to how many objects carried it. Absent when the
	// field turned out to have more distinct values than Cap allows, in which
	// case Distinct says how many there were and nothing is kept.
	Values map[string]int `json:"values,omitempty"`
	// Distinct is how many different values were seen, always.
	Distinct int `json:"distinct"`
	// Present is how many objects carried the field at all.
	Present int `json:"present"`
	// Suppressed says the values were dropped for being too many. A field that
	// turns out to be high-cardinality is a field that was mis-declared as
	// low-cardinality, and keeping its values anyway would be the copy this
	// forbids.
	Suppressed bool `json:"suppressed,omitempty"`
}

// StructureTallyCap is how many distinct values a recorded field may have
// before its values are dropped.
//
// Not a tuning parameter: it is the line between a distribution and a column.
// A modality has a handful of values; a date of birth has one per subject, and
// a "distribution" over it is the column with extra steps. Sixty-four is wide
// enough for every low-cardinality technical field seen so far and far too
// narrow to hold a per-subject value.
const StructureTallyCap = 64

// StructureScan is what one audited pass over a dataset's files found.
type StructureScan struct {
	// Allowlists is the list that was in force, stored rather than referenced,
	// for ADR-040's reason: a scan that cannot say what it was allowed to read
	// cannot be compared with the next one.
	Allowlists []StructureAllowlist `json:"allowlists"`
	// Objects is how many files were opened, and Formats how they broke down.
	Objects int            `json:"objects"`
	Formats map[string]int `json:"formats,omitempty"`
	// Tags is the set of fields each format carried, which is structure in its
	// purest form: the shape of the file, with no value attached.
	Tags map[string][]string `json:"tags,omitempty"`
	// Tallies are the recorded fields, as distributions.
	Tallies []FieldTally `json:"tallies,omitempty"`
	// IdentifyingPresent names the identifying fields that carried a value,
	// and IdentifyingChecked those that were looked at. Names only: the point
	// is which field betrayed the claim, never what it said.
	IdentifyingChecked []string `json:"identifyingChecked,omitempty"`
	IdentifyingPresent []string `json:"identifyingPresent,omitempty"`
	// Unreadable counts files that could not be parsed. Reported rather than
	// hidden: a scan that quietly skipped a thousand files and concluded
	// "no identifier found" would be worse than no scan.
	Unreadable int `json:"unreadable,omitempty"`
	// RanBy, At and Method are what makes a verdict datable. A fresh
	// comparison against a month-old scan is a month-old answer.
	RanBy  string    `json:"ranBy,omitempty"`
	At     time.Time `json:"at"`
	Method string    `json:"method,omitempty"`
}

// CarriedIdentifiers reports whether any identifying field held a value.
//
// This is the whole answer to "is this dataset pseudonymised", and it is
// deliberately a three-state answer: nil when nothing was checked, which is
// what keeps a card from claiming agreement with a check nobody ran.
func (s *StructureScan) CarriedIdentifiers() *bool {
	if s == nil || len(s.IdentifyingChecked) == 0 {
		return nil
	}
	carried := len(s.IdentifyingPresent) > 0
	return &carried
}

// Normalise sorts and dedupes what a scanner reports, so two scans of an
// unchanged dataset compare equal.
func (s *StructureScan) Normalise() {
	if s == nil {
		return
	}
	s.IdentifyingChecked = tidy(s.IdentifyingChecked)
	s.IdentifyingPresent = tidy(s.IdentifyingPresent)
	for format, tags := range s.Tags {
		s.Tags[format] = tidy(tags)
	}
	sort.Slice(s.Tallies, func(i, j int) bool { return s.Tallies[i].Field < s.Tallies[j].Field })
	for i := range s.Tallies {
		tally := &s.Tallies[i]
		if tally.Distinct == 0 {
			tally.Distinct = len(tally.Values)
		}
		// The cap is applied here and not only in the scanner, because the
		// scanner is a client: whatever it sends, a field with too many values
		// does not keep them.
		if len(tally.Values) > StructureTallyCap {
			tally.Values = nil
			tally.Suppressed = true
		}
	}
}

func tidy(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
