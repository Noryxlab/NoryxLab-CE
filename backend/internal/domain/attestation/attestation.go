// Package attestation holds what somebody has produced to say they were
// trained to handle confidential data.
//
// A document, not a decision. The platform records that a person uploaded a
// training certificate and when it stops being valid; it does not read it,
// does not verify it, and - for now, deliberately - does not stop anybody
// from doing anything. Stephane, 2026-11-27: "rien de coercitif pour
// l'instant."
//
// Recording it before enforcing it is the right order. An installation that
// starts refusing access on a field nobody has filled in locks out everybody
// on the first day; one that has been collecting the documents for months can
// see who is covered before it decides to require it.
//
// What it is not, said because the word invites it: not a certificate in the
// cryptographic sense. A PDF somebody was handed at the end of a course,
// scanned or exported - no signature to check, no chain to validate.
package attestation

import "time"

// Attestation is one uploaded document.
type Attestation struct {
	ID     string `json:"id"`
	UserID string `json:"userId"`
	// Subject is what the person says it covers: "HDS awareness", "RGPD",
	// the name of their employer's course. Free text because there is no
	// list anybody agrees on, and an enumeration invented here would be
	// wrong in the first installation that uses it.
	Subject  string `json:"subject"`
	FileName string `json:"fileName"`
	Size     int64  `json:"size"`
	// ValidUntil is optional and matters more than it looks. The Essilor
	// request of 2026-10-16 was explicit: a certificate that expires, not a
	// box that is ticked. A record with no expiry answers "did they ever do
	// the training"; one with an expiry answers "are they covered today".
	ValidUntil  *time.Time `json:"validUntil,omitempty"`
	UploadedAt  time.Time  `json:"uploadedAt"`
	StorageKey  string     `json:"-"`
	ContentType string     `json:"contentType"`
}

// Expired reports whether this document has passed its own date. A document
// with no date never expires, which is the honest reading of "they did not
// tell us when it runs out" - not an assumption that it is still good.
func (a Attestation) Expired(now time.Time) bool {
	return a.ValidUntil != nil && now.After(*a.ValidUntil)
}

// Status is the one word a screen shows, and the reason it is computed here
// rather than in two front ends that would drift.
func (a Attestation) Status(now time.Time) string {
	switch {
	case a.ValidUntil == nil:
		return "undated"
	case a.Expired(now):
		return "expired"
	default:
		return "valid"
	}
}
