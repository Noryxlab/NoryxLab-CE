package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
)

// Le droit sur un extrait suit son proprietaire, plus son auteur.
//
// Le controle comparait OwnerUserID - la personne qui a declare l extrait -
// ce qui etait la meme chose tant qu un extrait ne se transferait pas. Des
// qu il se transfere, les deux divergent : remettre un extrait a une
// organisation laissait a son auteur le droit de le supprimer, et le refusait
// a un membre de l organisation proprietaire.
func TestLeDroitSurUnExtraitSuitSonProprietaire(t *testing.T) {
	h := Handlers{}
	auteur := auth.Identity{Username: "alice", Subject: "alice"}

	// Transfere a quelqu un d autre : l auteur n a plus la main.
	transfere := extractdomain.Extract{
		ID: "e1", OwnerUserID: "alice", OwnerType: "user", OwnerID: "bob",
	}
	if h.canManageExtract(transfere, auteur) {
		t.Error("l auteur garde la main sur un extrait qu il a transfere")
	}

	// Chez lui : il l a.
	sien := extractdomain.Extract{
		ID: "e2", OwnerUserID: "alice", OwnerType: "user", OwnerID: "alice",
	}
	if !h.canManageExtract(sien, auteur) {
		t.Error("le proprietaire n a pas la main sur son propre extrait")
	}
}

// Et une ligne anterieure a la migration, sans proprietaire, reste a son
// auteur : mieux vaut lui qu a personne.
func TestUnExtraitSansProprietaireResteASonAuteur(t *testing.T) {
	h := Handlers{}
	ancien := extractdomain.Extract{ID: "e3", OwnerUserID: "alice"}
	if !h.canManageExtract(ancien, auth.Identity{Username: "alice", Subject: "alice"}) {
		t.Error("une ligne sans proprietaire a ete retiree a son auteur")
	}
	if h.canManageExtract(ancien, auth.Identity{Username: "bob", Subject: "bob"}) {
		t.Error("une ligne sans proprietaire est accessible a n importe qui")
	}
}
