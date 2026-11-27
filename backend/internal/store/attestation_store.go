package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/attestation"

// AttestationStore keeps what the platform knows about an uploaded training
// document. The document itself lives in object storage; this holds the row.
type AttestationStore interface {
	Create(item attestation.Attestation) error
	// ListByUser is newest first: a person's current document is the first
	// one, and the ones before it are the history that answers "were they
	// covered in March".
	ListByUser(userID string) ([]attestation.Attestation, error)
	// ListAll is what an administrator reads to see who is covered.
	ListAll() ([]attestation.Attestation, error)
	GetByID(id string) (attestation.Attestation, bool, error)
	Delete(id string) (bool, error)
}
