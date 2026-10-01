package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
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

// Renommer un extrait ne touche que son libelle.
//
// Le nom est tape a la declaration, donc il se tape mal, et il etait
// immuable : la seule issue etait de redeclarer la selection, ce qui fige une
// autre liste sur un bucket qui grandit - un renommage devenait une autre
// etude. Ce qui fait l'extrait, lui, ne bouge pas.
func TestRenommerUnExtraitNeTouchePasSaSelection(t *testing.T) {
	extraits := memory.NewExtractStore()
	item := extractdomain.Extract{
		ID: "e1", OntologyID: "o1", Name: "anterion-3-sujest", Description: "faute de frappe",
		OwnerUserID: "stef", OwnerType: "user", OwnerID: "stef",
		ObjectCount: 2940, TotalBytes: 6_410_000_000,
		Modalities: []string{"ANTERION"}, Subjects: []string{"S1", "S2", "S3"},
	}
	if err := extraits.Create(item, []extractdomain.Member{{ExtractID: "e1", Path: "S1/v1/ANTERION/a.e2e"}}); err != nil {
		t.Fatal(err)
	}
	if err := extraits.UpdateMetadata("e1", "anterion-3-sujets", "trois sujets, cornee"); err != nil {
		t.Fatal(err)
	}
	relu, found, err := extraits.GetByID("e1")
	if err != nil || !found {
		t.Fatalf("found=%v err=%v", found, err)
	}
	if relu.Name != "anterion-3-sujets" || relu.Description != "trois sujets, cornee" {
		t.Fatalf("libelle = %q / %q", relu.Name, relu.Description)
	}
	if relu.ObjectCount != 2940 || relu.TotalBytes != 6_410_000_000 {
		t.Fatalf("le n a bouge : %d objets, %d octets", relu.ObjectCount, relu.TotalBytes)
	}
	if relu.OwnerUserID != "stef" || relu.OwnerID != "stef" {
		t.Fatal("le proprietaire ou l'auteur a bouge")
	}
	membres, err := extraits.ListMembers("e1", 10)
	if err != nil || len(membres) != 1 {
		t.Fatalf("la liste figee a bouge : %d membre(s), err=%v", len(membres), err)
	}
}

// Et renommer un extrait qui n'existe pas se dit, au lieu de rendre un succes.
func TestRenommerUnExtraitInconnuEchoue(t *testing.T) {
	if err := memory.NewExtractStore().UpdateMetadata("absent", "x", ""); err == nil {
		t.Fatal("un renommage sans cible doit echouer")
	}
}
