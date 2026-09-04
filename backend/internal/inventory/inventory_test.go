package inventory

import "testing"

// An inventory that silently became a stub would read as a platform with no
// dependencies, which is the most reassuring possible answer and the most
// wrong. These assertions are what would have to fail first.
func TestTheEmbeddedInventoryIsRealAndParses(t *testing.T) {
	document, err := Parse()
	if err != nil {
		t.Fatalf("the embedded inventory does not parse: %v", err)
	}
	if len(document.Items) < 40 {
		t.Fatalf("only %d components: an inventory this short is a stub, not an answer", len(document.Items))
	}
	if document.GeneratedAt == "" {
		t.Fatal("no generation date: a compliance document with no date cannot be trusted")
	}
}

// Every component says where its licence came from. A list that mixes what was
// read from the dependency with what somebody typed here, without saying which
// is which, invites a reader to trust both equally.
func TestEveryComponentDeclaresItsOriginAndComponent(t *testing.T) {
	document, _ := Parse()
	valid := map[string]bool{"detected": true, "declared": true, "unresolved": true}
	for _, item := range document.Items {
		if item.Name == "" {
			t.Fatal("a component with no name")
		}
		if !valid[item.Origin] {
			t.Fatalf("%s: origin %q is not one of detected, declared, unresolved", item.Name, item.Origin)
		}
		if item.Component == "" {
			t.Fatalf("%s: no component, so a reader cannot tell what is deployed from what builds it", item.Name)
		}
	}
}

// The editions themselves must be in it: a customer asking what runs under the
// platform is asking about the platform too, not only its dependencies.
func TestTheEditionsAreListedWithTheirLicences(t *testing.T) {
	document, _ := Parse()
	found := map[string]string{}
	for _, item := range document.Items {
		if item.Component == "platform" {
			found[item.Name] = item.Licence
		}
	}
	if found["NoryxLab Community Edition"] != "MPL-2.0" {
		t.Fatalf("Community edition licence = %q, want MPL-2.0", found["NoryxLab Community Edition"])
	}
	if found["NoryxLab Enterprise Edition"] != "Proprietary" {
		t.Fatalf("Enterprise edition licence = %q, want Proprietary", found["NoryxLab Enterprise Edition"])
	}
}

// Reported, never guessed. A gap in a compliance document gets investigated; a
// guess gets believed.
func TestUnknownLicencesAreCountedRatherThanHidden(t *testing.T) {
	document, _ := Parse()
	if document.Counts["unknown"] != document.UnknownLicences() {
		t.Fatalf("the document claims %d unknown licences and contains %d",
			document.Counts["unknown"], document.UnknownLicences())
	}
}
