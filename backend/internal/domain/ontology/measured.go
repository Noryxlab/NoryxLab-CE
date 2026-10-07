package ontology

import "time"

// What the platform measured about this data, shown beside what a person
// declared (ADR-047).
//
// "La confiance n'exclut pas le contrôle." A card that only repeats what it was
// told is a brochure, so the declaration never appears alone: the figures the
// platform took itself sit next to it, and a reader judges.
//
// There are no verdicts here, and that is the simplification the card's own
// shape forced. The first version compared declared figures with measured ones
// field by field - agrees, differs, unverified, not checkable - which needed the
// person to have declared figures in the first place, under labels that turned
// out to be a hospital's vocabulary. With one paragraph there is nothing to
// compare: prose cannot be checked, which the old code said itself by answering
// "not checkable" for every prose field it was given.
//
// So the control is simpler and no weaker. Here is what somebody wrote, here is
// what we counted, here is when we counted it, and here is whether anything has
// ever looked inside the files. Nothing claims agreement with a check nobody
// ran.
type Measured struct {
	Subjects   int
	Objects    int64
	Modalities []string
	FirstVisit string
	LastVisit  string
	// Method and At describe the pass these came from. The date is of the
	// measurement, not of the reading: a fresh look at a month-old scan is a
	// month-old answer.
	Method string
	At     time.Time

	// IdentifyingFieldsSeen is what a structure scan found: whether any
	// identifying field carried a value. Nil when no scan has run, which is
	// the honest answer and the one the pseudonymisation question had nowhere
	// to record before - the survey of 2026-10-05 had to report 3 592 DICOM
	// headers unchecked.
	IdentifyingFieldsSeen *bool
	IdentifyingChecked    []string
	IdentifyingPresent    []string
	IdentifyingMethod     string
	IdentifyingAt         time.Time
}
