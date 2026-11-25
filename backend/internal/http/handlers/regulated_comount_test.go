package handlers

import (
	"slices"
	"testing"
)

func mountNamed(name, classification string, readOnly bool) workspaceAttachedDataset {
	return workspaceAttachedDataset{
		ID:             name,
		Name:           name,
		ReadOnly:       readOnly,
		Classification: classification,
	}
}

func readOnlyNames(items []workspaceAttachedDataset) []string {
	names := []string{}
	for _, item := range items {
		if item.ReadOnly {
			names = append(names, item.Name)
		}
	}
	return names
}

// The path this rule exists to close, in the shape it was found in: a
// regulated cohort read next to a personal dataset the same person owns, so
// one `cp` moves patient data somewhere it can be downloaded legitimately.
func TestAPersonalDatasetBesideRegulatedDataIsReadOnly(t *testing.T) {
	got := applyRegulatedCoMountRule([]workspaceAttachedDataset{
		mountNamed("hds-for", "hds", true),
		mountNamed("dataset-local-stef", "", false),
	}, false)
	if !got[1].ReadOnly {
		t.Fatal("the personal dataset stayed writable beside regulated data")
	}
	if got[1].ReadOnlyReason != readOnlyReasonRegulatedCoMount {
		t.Fatalf("no reason recorded: %q", got[1].ReadOnlyReason)
	}
	if names := readOnlyForCoMount(got); !slices.Equal(names, []string{"dataset-local-stef"}) {
		t.Fatalf("the launch message names %v", names)
	}
}

// Nothing changes for a project with no regulated data, which is 16 of the 27
// projects on EMSE. A rule that quietly made every mount read-only would be a
// platform outage wearing a control's name.
func TestWithoutRegulatedDataNothingIsDemoted(t *testing.T) {
	got := applyRegulatedCoMountRule([]workspaceAttachedDataset{
		mountNamed("scratch", "", false),
		mountNamed("public-images", "", false),
	}, false)
	if names := readOnlyNames(got); len(names) != 0 {
		t.Fatalf("demoted %v in a project that holds nothing regulated", names)
	}
	if names := readOnlyForCoMount(got); len(names) != 0 {
		t.Fatalf("the launch message names %v with no rule applied", names)
	}
}

// Writing from one regulated dataset into another is the normal case for a
// researcher, and it does not leave the perimeter: the destination carries the
// same classification, so its own project is closed and its bulk download is
// refused in turn. Demoting it would push the work towards the unclassified
// dataset this rule exists to protect.
func TestARegulatedDestinationStaysWritable(t *testing.T) {
	got := applyRegulatedCoMountRule([]workspaceAttachedDataset{
		mountNamed("selena", "hds", true),
		mountNamed("selena-results", "hds", false),
	}, false)
	if got[1].ReadOnly {
		t.Fatal("a regulated destination was demoted, which sends the write elsewhere")
	}
	if names := readOnlyForCoMount(got); len(names) != 0 {
		t.Fatalf("a message was produced for a rule that did not apply: %v", names)
	}
}

// A mount the person cannot write anyway is not the rule's doing, and must not
// be announced as such: the message would send somebody looking for a right
// they never had.
func TestAMountAlreadyReadOnlyIsNotClaimedByTheRule(t *testing.T) {
	got := applyRegulatedCoMountRule([]workspaceAttachedDataset{
		mountNamed("hds-for", "hds", true),
		mountNamed("someone-elses", "", true),
	}, false)
	if got[1].ReadOnlyReason != "" {
		t.Fatalf("the rule claimed a mount the right had already closed: %q", got[1].ReadOnlyReason)
	}
	if names := readOnlyForCoMount(got); len(names) != 0 {
		t.Fatalf("the launch message names %v", names)
	}
}

// The classification is compared the way the rest of the platform compares it -
// trimmed and case-insensitive - because it is a stored string and a row
// written by hand reads "HDS" just as truthfully.
func TestTheClassificationIsReadLooselyEnough(t *testing.T) {
	got := applyRegulatedCoMountRule([]workspaceAttachedDataset{
		mountNamed("cohort", " HDS ", true),
		mountNamed("scratch", "", false),
	}, false)
	if !got[1].ReadOnly {
		t.Fatal("an upper-case classification did not trigger the rule")
	}
}

// An empty list and a list of one must not panic or invent a mount: both are
// ordinary - a project attaches nothing, or attaches exactly the cohort.
func TestTheEdgesAreQuiet(t *testing.T) {
	if got := applyRegulatedCoMountRule(nil, false); len(got) != 0 {
		t.Fatalf("nil produced %v", got)
	}
	single := applyRegulatedCoMountRule([]workspaceAttachedDataset{mountNamed("hds-for", "hds", false)}, false)
	if single[0].ReadOnly {
		t.Fatal("the only regulated dataset demoted itself")
	}
}

// La fonction de la matrice, decidee le 2026-11-25 : la ligne qui la porte
// garde ses montages inscriptibles, et c'est le croisement d'empreintes de
// nuit qui prend le relais.
func TestLaFonctionDeLaMatriceRendLEcritureACoteDuReglemente(t *testing.T) {
	mounts := []workspaceAttachedDataset{
		mountNamed("hds-for", "hds", true),
		mountNamed("dataset-local-stef", "", false),
	}
	got := applyRegulatedCoMountRule(mounts, true)
	if len(readOnlyNames(got)) != 1 || readOnlyNames(got)[0] != "hds-for" {
		t.Fatalf("seule la cohorte devait rester en lecture seule, obtenu %v", readOnlyNames(got))
	}
	if got[1].ReadOnlyReason != "" {
		t.Fatalf("une exemption ne doit laisser aucune raison derriere elle: %q", got[1].ReadOnlyReason)
	}
	if len(readOnlyForCoMount(got)) != 0 {
		t.Fatalf("rien n'a ete pris par la regle, le message de lancement doit se taire: %v", readOnlyForCoMount(got))
	}
}

// L'exemption ne touche pas une lecture seule qui vient d'ailleurs : un droit
// Read sur la donnee reste un droit Read, la fonction ne le promeut pas.
func TestLExemptionNePromeutPasUnDroitDeLecture(t *testing.T) {
	got := applyRegulatedCoMountRule([]workspaceAttachedDataset{
		mountNamed("hds-for", "hds", false),
		mountNamed("partage-en-lecture", "", true),
	}, true)
	if !got[1].ReadOnly {
		t.Fatal("l'exemption a rendu inscriptible un montage qui ne l'etait pas")
	}
}
