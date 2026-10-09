package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/settings"
)

// The setting has exactly two answers, and the default is the behaviour every
// existing installation already has.
//
// A new setting that changes what an installation does the moment it ships is
// a new setting that breaks somebody's platform on upgrade.
func TestLocalDownloadDefaultsToTodaysBehaviour(t *testing.T) {
	definition, found := settings.Lookup(settings.KeyDatasetLocalDownload)
	if !found {
		t.Fatal("the setting must be declared: an undeclared key is refused by the API")
	}
	if definition.Fallback != settings.DatasetDownloadAllowed {
		t.Errorf("fallback = %q, want %q", definition.Fallback, settings.DatasetDownloadAllowed)
	}
	if definition.Kind != settings.KindEnum {
		t.Errorf("kind = %q: a free-form value here would be a typo that silently allows", definition.Kind)
	}
	// Enumerated, so "disabled" or "none" is refused rather than stored and
	// read as "not blocked".
	want := map[string]bool{settings.DatasetDownloadAllowed: true, settings.DatasetDownloadBlocked: true}
	if len(definition.Values) != len(want) {
		t.Fatalf("values = %v", definition.Values)
	}
	for _, value := range definition.Values {
		if !want[value] {
			t.Errorf("unexpected value %q", value)
		}
	}
}

// An installation with no settings store downloads as before.
//
// The community edition can run without one, and a nil store must read as
// "allowed" rather than panicking or, worse, as "blocked" - which would make
// the absence of configuration look like a deliberate lockdown.
func TestNoSettingsStoreMeansDownloadsStayOn(t *testing.T) {
	if (Handlers{}).localDownloadBlocked() {
		t.Error("with no settings store, downloads must stay allowed")
	}
	if got := (Handlers{}).currentDatasetDownload(); got != settings.DatasetDownloadAllowed {
		t.Errorf("currentDatasetDownload() = %q", got)
	}
}

// What still comes through when downloads are withdrawn: what the viewer can
// display, and nothing else.
//
// The object route serves previews as well as saving a file and the request
// does not say which, so withdrawal narrows that route instead of closing it.
// Closing it would take the viewer away, which nobody asked for.
func TestWithdrawingDownloadsKeepsThePreviewAndStopsTheRest(t *testing.T) {
	for _, path := range []string{
		"notes.pdf", "mesures.csv", "rapport.xlsx", "coupe.png",
		"sujet-001/visite/lettre.PDF", "a/b/c/table.json",
	} {
		if !previewableType(path) {
			t.Errorf("%q must stay readable in the interface", path)
		}
	}
	for _, path := range []string{
		"IRM/coupe.dcm", "archive.zip", "serie.e2e", "modele.nii.gz",
		"donnees.parquet", "script.sh", "sansextension",
	} {
		if previewableType(path) {
			t.Errorf("%q must not come out through the preview route", path)
		}
	}
}

// Previewing is not limited to the dataset's root, unlike HDS.
//
// HDS allows a root document only, deliberately. Applying that rule to an
// ordinary dataset would make a platform whose files cannot be read in any
// folder, which is not what withdrawing downloads means.
func TestOrdinaryPreviewsAreNotLimitedToTheRoot(t *testing.T) {
	nested := "PREMYOM1000/sujet-001/20260218/mesures.csv"
	if !previewableType(nested) {
		t.Error("an ordinary dataset must be readable in its folders")
	}
	if hdsRootDocumentPreviewAllowed(nested) {
		t.Error("HDS keeps its stricter root-only rule")
	}
}
