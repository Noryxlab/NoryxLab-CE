package ontology

import (
	"strings"
	"time"
)

// What a dataset says about itself, in a person's words (ADR-047).
//
// The ontology scan produces an inventory of paths: subjects, visits,
// modalities, counts, volumes. Useful, honest about its method, and silent on
// everything that matters to somebody who did not build the dataset - what it
// is about, what it may be used for, what it must not be used for, which
// version of the cohort it is, under what legal basis it was collected, who to
// ask when the answer is not in the file.
//
// None of that is inferable from any number of bytes, which is the finding
// behind this type: a card is largely declared rather than measured, so the
// larger half of the gap was a vocabulary gap. This file is that vocabulary.
//
// It lives on the ontology, which is where meaning belongs: a dataset is a
// bucket with credentials, and describing what the data *means* on the storage
// object is a category error - it also forces one description on every ontology
// that reads the same bucket, and a bucket can carry several studies read
// several ways.
//
// It was first put on the dataset, on the reasoning that a rescan would lose a
// card living on the ontology. That reading of ADR-043 was wrong: a rescan
// "replaces the picture and keeps the object - its identifier, its name, its
// owner and the extracts that point at it". The manifest is replaced; the
// ontology row is not. A card stored beside the manifest survives a rescan
// exactly as the name does.
type Card struct {
	// Version increments on every accepted edit, and an extract records the
	// version it was cut against. Without it a cohort's n becomes
	// unexplainable the moment the card moves under it.
	Version int `json:"version"`

	// What it is.
	Study      string `json:"study,omitempty"`
	Release    string `json:"release,omitempty"`
	Population string `json:"population,omitempty"`
	Inclusion  string `json:"inclusion,omitempty"`

	// What it is for, and what it is not for.
	//
	// Both, and the second is not the negation of the first. A dataset
	// collected to measure progression may be perfectly usable for a method
	// paper and unusable for a screening claim, and only its owner knows that.
	Purpose     string `json:"purpose,omitempty"`
	OutOfScope  string `json:"outOfScope,omitempty"`
	Limitations string `json:"limitations,omitempty"`

	// Where it comes from and under what right it is held.
	Provenance string `json:"provenance,omitempty"`
	LegalBasis string `json:"legalBasis,omitempty"`
	Consent    string `json:"consent,omitempty"`
	Licence    string `json:"licence,omitempty"`

	// How to read a measurement in it. Units and conventions are the field
	// that turns a number into a fact, and the one nobody writes down.
	Units       string `json:"units,omitempty"`
	Conventions string `json:"conventions,omitempty"`

	// Who to ask. A card without this sends the reader back to asking around,
	// which is the problem.
	Contact string `json:"contact,omitempty"`

	// Claims are the declared figures that can be checked against what the
	// platform measures. Separate from the prose above because prose cannot be
	// checked and these can.
	Claims Claims `json:"claims,omitempty"`

	// Who last wrote this and when. A declaration is somebody's word, so it
	// carries whose.
	DeclaredBy string    `json:"declaredBy,omitempty"`
	DeclaredAt time.Time `json:"declaredAt,omitempty"`
}

// Claims is the checkable part of a declaration.
//
// Trust does not exclude verification. A card that only repeats what it was
// told is a brochure, so the figures a person states are kept apart from the
// prose and compared with the inventory - reported beside the declaration,
// never written over it, because replacing somebody's word with a measurement
// destroys the only evidence that they disagreed.
//
// Pointers, so "said nothing" is distinguishable from "said zero". A dataset
// declaring zero subjects is a mistake worth showing; a dataset declaring
// nothing is simply undeclared.
type Claims struct {
	Subjects   *int     `json:"subjects,omitempty"`
	Objects    *int64   `json:"objects,omitempty"`
	Modalities []string `json:"modalities,omitempty"`
	// FirstVisit and LastVisit are AAAAMMJJ, the form the keys already use.
	FirstVisit string `json:"firstVisit,omitempty"`
	LastVisit  string `json:"lastVisit,omitempty"`
	// Pseudonymised is the claim the pseudonymisation survey of 2026-10-05 had
	// nowhere to record. Checking it needs the structure scan; until that has
	// run the verdict is "unverified", which is the honest answer and the
	// reason this field exists at all.
	Pseudonymised *bool `json:"pseudonymised,omitempty"`
}

// Declared reports whether anybody has written anything. An empty card is a
// legitimate state and is shown as empty: an unanswered field is information,
// and filling it with a guess is the failure ADR-047 exists to avoid.
func (c *Card) Declared() bool {
	if c == nil {
		return false
	}
	for _, text := range []string{
		c.Study, c.Release, c.Population, c.Inclusion,
		c.Purpose, c.OutOfScope, c.Limitations,
		c.Provenance, c.LegalBasis, c.Consent, c.Licence,
		c.Units, c.Conventions, c.Contact,
	} {
		if strings.TrimSpace(text) != "" {
			return true
		}
	}
	return c.Claims.Declared()
}

// Declared reports whether any checkable figure was stated.
func (cl Claims) Declared() bool {
	return cl.Subjects != nil || cl.Objects != nil || len(cl.Modalities) > 0 ||
		strings.TrimSpace(cl.FirstVisit) != "" || strings.TrimSpace(cl.LastVisit) != "" ||
		cl.Pseudonymised != nil
}

// Normalise trims what a form sends and drops what it sent empty, so a card
// edited twice with the same content does not differ in storage.
func (c *Card) Normalise() {
	if c == nil {
		return
	}
	for _, field := range []*string{
		&c.Study, &c.Release, &c.Population, &c.Inclusion,
		&c.Purpose, &c.OutOfScope, &c.Limitations,
		&c.Provenance, &c.LegalBasis, &c.Consent, &c.Licence,
		&c.Units, &c.Conventions, &c.Contact,
		&c.Claims.FirstVisit, &c.Claims.LastVisit,
	} {
		*field = strings.TrimSpace(*field)
	}
	modalities := make([]string, 0, len(c.Claims.Modalities))
	seen := map[string]bool{}
	for _, modality := range c.Claims.Modalities {
		modality = strings.TrimSpace(modality)
		if modality == "" || seen[strings.ToLower(modality)] {
			continue
		}
		seen[strings.ToLower(modality)] = true
		modalities = append(modalities, modality)
	}
	c.Claims.Modalities = modalities
}
