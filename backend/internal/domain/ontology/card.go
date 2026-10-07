package ontology

import (
	"strings"
	"time"
)

// What an ontology says the data means, in a person's words (ADR-047).
//
// The scan produces an inventory of paths: subjects, visits, modalities,
// counts, volumes. Useful, honest about its method, and silent on everything
// that matters to somebody who did not build the dataset - what it is about,
// what it may be used for, what it must not be used for, under what right it
// is held, who to ask. None of that is inferable from any number of bytes,
// which is why it is declared.
//
// One field, and that is the whole design.
//
// The first version had fourteen labelled ones - study, population, inclusion
// criteria, consent, purpose, out-of-scope, limitations, provenance, legal
// basis, licence, units, conventions, contact, release - plus six checkable
// figures. Two things were wrong with it. A form that long is a form nobody
// fills, so it would have collected fourteen empty fields instead of one
// paragraph somebody actually wrote. And half the labels were a hospital's
// vocabulary on a platform that is also sold to banks and to Inria: population
// and visits and modalities mean nothing over a table of transactions, and a
// generic platform whose only description form assumes a clinical trial is not
// generic.
//
// A paragraph is domain-neutral by construction. It also happens to be what
// people write when you ask them what a dataset is.
type Card struct {
	// Version increments on every accepted edit, and an extract records the
	// version it was cut against. Without it a cohort's n becomes
	// unexplainable the moment the card moves under it.
	Version int `json:"version"`
	// Text is the declaration itself.
	Text string `json:"text,omitempty"`
	// Who wrote it and when. A declaration is somebody's word, so it carries
	// whose - which is also what tells a reader how old the word is.
	DeclaredBy string    `json:"declaredBy,omitempty"`
	DeclaredAt time.Time `json:"declaredAt,omitempty"`
}

// Declared reports whether anybody has written anything.
//
// Empty is a legitimate state and is shown as empty: an unanswered field is
// information, and filling it with a guess is the failure this exists to avoid.
func (c *Card) Declared() bool {
	return c != nil && strings.TrimSpace(c.Text) != ""
}

// Normalise trims what a form sends, so a card edited twice with the same
// content does not differ in storage.
func (c *Card) Normalise() {
	if c == nil {
		return
	}
	c.Text = strings.TrimSpace(c.Text)
}
