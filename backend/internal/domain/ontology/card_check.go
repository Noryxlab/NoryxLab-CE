package ontology

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Checking a declaration against what was measured (ADR-047).
//
// "La confiance n'exclut pas le contrôle." A declared card is somebody's word,
// and a card that only repeats it is a brochure. So every figure a person
// states and the platform can measure is compared - and three rules keep that a
// control rather than a nag.
//
// A check reports, it never overwrites. The declaration stays as written and the
// verdict sits beside it, because replacing somebody's word with a measurement
// destroys the only evidence that they disagreed - which is the interesting
// fact, not the number.
//
// A check carries its date and its method. "Pseudonymisation: declared yes,
// unverified" and "declared yes, checked against 42 DICOM tags on 2026-10-07,
// no identifying field carried a value" are different sentences and must read
// differently.
//
// Unverifiable is a verdict. Intended use and legal basis cannot be measured by
// anything; saying so is honest, and inventing a proxy for them would be the
// kind of claim ADR-034 forbids.

// Verdict is what a check concluded.
type Verdict string

const (
	// VerdictAgrees - the measurement matches the declaration.
	VerdictAgrees Verdict = "agrees"
	// VerdictDiffers - both are known and they do not match. Not an error:
	// a stale declaration and a wrong reading rule look identical here, and
	// which it is belongs to a person.
	VerdictDiffers Verdict = "differs"
	// VerdictUndeclared - nothing was stated, so there is nothing to check.
	VerdictUndeclared Verdict = "undeclared"
	// VerdictUnverified - stated, but nothing has measured it yet. The
	// pseudonymisation claim sits here until the structure scan has run.
	VerdictUnverified Verdict = "unverified"
	// VerdictNotCheckable - nothing could measure this, ever.
	VerdictNotCheckable Verdict = "not_checkable"
)

// Check is one line of the card's control column.
type Check struct {
	Field    string  `json:"field"`
	Verdict  Verdict `json:"verdict"`
	Declared string  `json:"declared,omitempty"`
	Measured string  `json:"measured,omitempty"`
	// Method is how the measurement was obtained, named so a reader can judge
	// it: "path scan", "structure scan", "inventory".
	Method string `json:"method,omitempty"`
	// At is when the measurement this compares against was taken, not when the
	// comparison ran. A fresh comparison against a month-old scan is a
	// month-old answer.
	At time.Time `json:"at,omitempty"`
	// Note explains a verdict a figure cannot: why it is not checkable, or
	// what a difference most likely means.
	Note string `json:"note,omitempty"`
}

// Measured is what the platform knows from its own passes over the dataset.
//
// Separate from the card so the comparison takes two arguments and keeps no
// state: whatever produced these figures - a path scan, an inventory, later a
// structure scan - the comparison does not need to know.
type Measured struct {
	Subjects   int
	Objects    int64
	Modalities []string
	FirstVisit string
	LastVisit  string
	// Method and At describe the pass these came from.
	Method string
	At     time.Time
	// IdentifyingFieldsSeen is set by a structure scan: the allowlisted
	// identifying-field check found values where there should be none. Nil
	// means no structure scan has run, which is why the pseudonymisation
	// verdict is "unverified" rather than "agrees".
	IdentifyingFieldsSeen *bool
	IdentifyingMethod     string
	IdentifyingAt         time.Time
}

// CheckCard compares a declaration with a measurement, field by field.
//
// Every checkable field produces a line, including the ones nobody declared:
// a card showing "subjects: undeclared" tells a reader something, and a card
// that silently omits what was not declared tells them nothing.
func CheckCard(card *Card, measured Measured) []Check {
	checks := make([]Check, 0, 6)
	claims := Claims{}
	if card != nil {
		claims = card.Claims
	}

	checks = append(checks, compareInt("subjects", claims.Subjects, measured.Subjects,
		measured.Method, measured.At,
		"a stale declaration and a reading rule that misses subjects look identical here"))

	var declaredObjects *int64
	if claims.Objects != nil {
		declaredObjects = claims.Objects
	}
	checks = append(checks, compareInt64("objects", declaredObjects, measured.Objects,
		measured.Method, measured.At,
		"files added outside the declared process arrive here first"))

	checks = append(checks, compareSet("modalities", claims.Modalities, measured.Modalities,
		measured.Method, measured.At))

	checks = append(checks, compareText("firstVisit", claims.FirstVisit, measured.FirstVisit,
		measured.Method, measured.At))
	checks = append(checks, compareText("lastVisit", claims.LastVisit, measured.LastVisit,
		measured.Method, measured.At,
	))

	checks = append(checks, checkPseudonymisation(claims.Pseudonymised, measured))

	// The prose. Named rather than omitted, because "nothing could check this"
	// is the honest thing to say about a purpose, and a reader who does not see
	// the field cannot tell it apart from a field that passed.
	for _, field := range []string{"purpose", "outOfScope", "limitations", "legalBasis", "consent", "licence"} {
		declared := prose(card, field)
		verdict := VerdictNotCheckable
		if declared == "" {
			verdict = VerdictUndeclared
		}
		checks = append(checks, Check{
			Field: field, Verdict: verdict, Declared: declared,
			Note: "nothing the platform measures can confirm or contradict this",
		})
	}
	return checks
}

func checkPseudonymisation(declared *bool, measured Measured) Check {
	check := Check{Field: "pseudonymised"}
	if declared == nil {
		check.Verdict = VerdictUndeclared
		return check
	}
	check.Declared = fmt.Sprintf("%t", *declared)
	if measured.IdentifyingFieldsSeen == nil {
		check.Verdict = VerdictUnverified
		check.Note = "no structure scan has read this dataset's technical fields yet"
		return check
	}
	check.Method = measured.IdentifyingMethod
	check.At = measured.IdentifyingAt
	seen := *measured.IdentifyingFieldsSeen
	check.Measured = fmt.Sprintf("identifying fields carrying a value: %t", seen)
	// Declared pseudonymised and identifiers found is the one combination that
	// matters, and it is a difference rather than an error here: this type
	// reports, somebody decides.
	if *declared == !seen {
		check.Verdict = VerdictAgrees
	} else {
		check.Verdict = VerdictDiffers
	}
	return check
}

func compareInt(field string, declared *int, measured int, method string, at time.Time, note string) Check {
	if declared == nil {
		return Check{Field: field, Verdict: VerdictUndeclared,
			Measured: fmt.Sprint(measured), Method: method, At: at}
	}
	check := Check{Field: field, Declared: fmt.Sprint(*declared),
		Measured: fmt.Sprint(measured), Method: method, At: at}
	if *declared == measured {
		check.Verdict = VerdictAgrees
		return check
	}
	check.Verdict = VerdictDiffers
	check.Note = note
	return check
}

func compareInt64(field string, declared *int64, measured int64, method string, at time.Time, note string) Check {
	if declared == nil {
		return Check{Field: field, Verdict: VerdictUndeclared,
			Measured: fmt.Sprint(measured), Method: method, At: at}
	}
	check := Check{Field: field, Declared: fmt.Sprint(*declared),
		Measured: fmt.Sprint(measured), Method: method, At: at}
	if *declared == measured {
		check.Verdict = VerdictAgrees
		return check
	}
	check.Verdict = VerdictDiffers
	check.Note = note
	return check
}

func compareText(field, declared, measured, method string, at time.Time) Check {
	declared = strings.TrimSpace(declared)
	if declared == "" {
		return Check{Field: field, Verdict: VerdictUndeclared,
			Measured: measured, Method: method, At: at}
	}
	check := Check{Field: field, Declared: declared, Measured: measured, Method: method, At: at}
	switch {
	case measured == "":
		check.Verdict = VerdictUnverified
		check.Note = "no pass has produced this figure yet"
	case declared == measured:
		check.Verdict = VerdictAgrees
	default:
		check.Verdict = VerdictDiffers
	}
	return check
}

// compareSet compares two sets of names, case-insensitively, and says which way
// they differ - "a modality arrived" and "a modality left" call for different
// conversations.
func compareSet(field string, declared, measured []string, method string, at time.Time) Check {
	if len(declared) == 0 {
		return Check{Field: field, Verdict: VerdictUndeclared,
			Measured: strings.Join(sorted(measured), ", "), Method: method, At: at}
	}
	check := Check{Field: field,
		Declared: strings.Join(sorted(declared), ", "),
		Measured: strings.Join(sorted(measured), ", "),
		Method:   method, At: at}
	if len(measured) == 0 {
		check.Verdict = VerdictUnverified
		check.Note = "no pass has produced this set yet"
		return check
	}
	inMeasured := map[string]bool{}
	for _, name := range measured {
		inMeasured[strings.ToLower(strings.TrimSpace(name))] = true
	}
	inDeclared := map[string]bool{}
	for _, name := range declared {
		inDeclared[strings.ToLower(strings.TrimSpace(name))] = true
	}
	var unannounced, missing []string
	for name := range inMeasured {
		if !inDeclared[name] {
			unannounced = append(unannounced, name)
		}
	}
	for name := range inDeclared {
		if !inMeasured[name] {
			missing = append(missing, name)
		}
	}
	if len(unannounced) == 0 && len(missing) == 0 {
		check.Verdict = VerdictAgrees
		return check
	}
	check.Verdict = VerdictDiffers
	var parts []string
	if len(unannounced) > 0 {
		parts = append(parts, "present but not declared: "+strings.Join(sorted(unannounced), ", "))
	}
	if len(missing) > 0 {
		parts = append(parts, "declared but absent: "+strings.Join(sorted(missing), ", "))
	}
	check.Note = strings.Join(parts, "; ")
	return check
}

func prose(card *Card, field string) string {
	if card == nil {
		return ""
	}
	switch field {
	case "purpose":
		return card.Purpose
	case "outOfScope":
		return card.OutOfScope
	case "limitations":
		return card.Limitations
	case "legalBasis":
		return card.LegalBasis
	case "consent":
		return card.Consent
	case "licence":
		return card.Licence
	}
	return ""
}

func sorted(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}
