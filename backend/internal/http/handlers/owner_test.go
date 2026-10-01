package handlers

import (
	"net/http"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
)

// Un seul valideur, parce que quatre copies avaient deja derive.
//
// Un dataset connaissait les equipes comme sujets d acces et pas une
// ontologie, donc une ontologie ne pouvait meme pas etre partagee avec une
// equipe. "Pareil partout" est une propriete du code avant d etre une
// propriete du produit.
func TestCeQuUnProprietairePeutEtre(t *testing.T) {
	h := Handlers{}
	moi := auth.Identity{Username: "alice", Subject: "alice"}

	cas := []struct {
		nom     string
		kind    string
		id      string
		accepte bool
		status  int
	}{
		{"une personne", "user", "alice", true, 0},
		{"la casse est sans importance", "USER", "alice", true, 0},
		{"un type inconnu", "robot", "r2", false, http.StatusBadRequest},
		{"un type vide", "", "alice", false, http.StatusBadRequest},
		{"un identifiant vide", "user", "", false, http.StatusBadRequest},
		{"un identifiant d espaces", "user", "   ", false, http.StatusBadRequest},
	}
	for _, c := range cas {
		t.Run(c.nom, func(t *testing.T) {
			kind, id, status, probleme := h.normaliseOwner(c.kind, c.id, moi)
			if c.accepte {
				if status != 0 {
					t.Fatalf("refuse (%d) : %s", status, probleme)
				}
				if kind != "user" || id != "alice" {
					t.Errorf("normalise en %q/%q", kind, id)
				}
				return
			}
			if status != c.status {
				t.Errorf("status %d, attendu %d (%s)", status, c.status, probleme)
			}
			if probleme == "" {
				t.Error("un refus sans message")
			}
		})
	}
}

// Le message doit nommer les trois possibilites : il est lu par quelqu un qui
// vient de se tromper, et "ownerType invalide" ne lui apprend rien.
func TestLeRefusNommeLesTroisPossibilites(t *testing.T) {
	_, _, _, probleme := Handlers{}.normaliseOwner("robot", "r2", auth.Identity{})
	for _, attendu := range []string{"user", "team", "organization"} {
		if !strings.Contains(probleme, attendu) {
			t.Errorf("le refus ne mentionne pas %q : %s", attendu, probleme)
		}
	}
}
