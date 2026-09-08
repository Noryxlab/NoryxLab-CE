package handlers

import "testing"

func subjectWith(id string, modalities ...string) ontologySubject {
	visit := ontologyVisit{Date: "2026-01-01"}
	for _, name := range modalities {
		visit.Modalities = append(visit.Modalities, ontologyModality{Name: name, ObjectCount: 10})
	}
	return ontologySubject{ID: id, Visits: []ontologyVisit{visit}}
}

// The half that was missing: not how many objects a modality holds, but which
// subjects do not have it. A cohort asked for by modality quietly excludes them.
func TestCompletenessNamesTheSubjectsAModalityMisses(t *testing.T) {
	manifest := ontologyManifest{Subjects: []ontologySubject{
		subjectWith("S1", "Cornea_Basics", "Cornea_Wavefront"),
		subjectWith("S2", "Cornea_Basics"),
		subjectWith("S3", "Cornea_Basics"),
	}}

	report := computeOntologyCompleteness(manifest)
	if report.Subjects != 3 {
		t.Fatalf("subjects = %d, want 3", report.Subjects)
	}
	// Sparsest first: the modality that decides whether a cohort is viable.
	if report.Modalities[0].Name != "Cornea_Wavefront" {
		t.Fatalf("first modality = %q, want the sparsest one", report.Modalities[0].Name)
	}
	if got := report.Modalities[0].MissingSubjects; len(got) != 2 || got[0] != "S2" || got[1] != "S3" {
		t.Fatalf("missing subjects = %v, want [S2 S3]", got)
	}
	if report.CompleteSubjects != 1 {
		t.Fatalf("completeSubjects = %d, want 1", report.CompleteSubjects)
	}
}

// A modality every subject carries reports no gap - and an empty list, never a
// null the screen would have to guess at.
func TestCompletenessReportsNoGapWhenEveryoneHasIt(t *testing.T) {
	manifest := ontologyManifest{Subjects: []ontologySubject{
		subjectWith("S1", "Cornea_Basics"),
		subjectWith("S2", "Cornea_Basics"),
	}}
	report := computeOntologyCompleteness(manifest)
	if len(report.Modalities) != 1 || len(report.Modalities[0].MissingSubjects) != 0 {
		t.Fatalf("unexpected gaps: %+v", report.Modalities)
	}
	if report.CompleteSubjects != 2 {
		t.Fatalf("completeSubjects = %d, want 2", report.CompleteSubjects)
	}
}
