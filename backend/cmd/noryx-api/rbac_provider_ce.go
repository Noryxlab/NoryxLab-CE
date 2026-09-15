//go:build !enterprise

package main

import (
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/edition"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

// editionRBACProvider supplies the permission engine the edition brings.
//
// Community has none: its rule is the one written in the handlers, and the
// stored role matrix is a document it does not consult. Returning nil here is
// what makes that explicit, rather than leaving the field unset in a
// constructor nobody reads to the end.
func editionRBACProvider(_ store.RBACPolicyStore, _ func(string) bool) edition.RBACProvider {
	return nil
}
