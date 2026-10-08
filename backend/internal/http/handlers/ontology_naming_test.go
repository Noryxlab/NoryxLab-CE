package handlers

import "testing"

// The study's name is read, not derived from a patient identifier.
//
// It used to be the first subject id with everything after the last dash
// removed. PREMYOM1000-0001 gave PREMYOM1000, which was right by luck, so the
// rule shipped. SELENA-01-001 gives SELENA-01 - the investigating centre, not
// the study - and the catalogue displayed a centre code as the name of a
// trial, on every screen, for every extract cut from it.
func TestTheStudyNameIsReadFromThePathNotChoppedFromAnIdentifier(t *testing.T) {
	cases := []struct {
		name    string
		path    string
		subject string
		want    string
	}{
		{
			name:    "three-segment identifiers under a study directory",
			path:    "SELENA/SELENA-01-001/20260218/ANTERION/scan.dcm",
			subject: "SELENA-01-001",
			want:    "SELENA",
		},
		{
			name:    "two-segment identifiers under a study directory",
			path:    "PREMYOM1000/PREMYOM1000-0001/20260115/IRM/scan.dcm",
			subject: "PREMYOM1000-0001",
			want:    "PREMYOM1000",
		},
		{
			// Nothing is invented: the caller falls back to the dataset's
			// own name rather than guessing at the identifier's shape.
			name:    "subjects at the root name no study",
			path:    "PREMYOM1000-0001/20260115/IRM/scan.dcm",
			subject: "PREMYOM1000-0001",
			want:    "",
		},
		{
			name:    "a subject the path does not carry",
			path:    "checksums/checksum_20260918.tsv",
			subject: "SELENA-01-001",
			want:    "",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := studyFromPath(testCase.path, testCase.subject); got != testCase.want {
				t.Errorf("studyFromPath(%q, %q) = %q, want %q",
					testCase.path, testCase.subject, got, testCase.want)
			}
		})
	}
}

// The platform's fallback words never travel in a manifest.
//
// describeReading used to send "entity", "period" and "category" for an
// undeclared layout, so the screen's own localised fallbacks were never
// reached and a French page displayed "entity" - then "entitys", because the
// plural was an s glued onto a word nobody had chosen. A name belongs to the
// manifest only when somebody declared it; the rest is a label, and a label
// belongs to whoever renders it.
func TestAnUndeclaredLayoutCarriesNoNames(t *testing.T) {
	rule := describeReading(nil)
	if rule.Source != "default" {
		t.Fatalf("source = %q, want default", rule.Source)
	}
	if rule.SubjectName != "" || rule.VisitName != "" || rule.ModalityName != "" {
		t.Errorf("an undeclared layout named its levels: %q / %q / %q",
			rule.SubjectName, rule.VisitName, rule.ModalityName)
	}
}
