package team

import (
	"errors"
	"strings"
	"testing"
)

func TestUnNomVideNEstPasUnNom(t *testing.T) {
	for _, brut := range []string{"", "   ", "\t\n"} {
		equipe := Team{Name: brut, OrganizationID: "org-1"}
		if err := equipe.Normalise(); !errors.Is(err, ErrNameRequired) {
			t.Errorf("%q accepte : %v", brut, err)
		}
	}
}

// Une equipe sans organisation n'a personne pour en repondre, et l'octroi
// qu'elle porte survivrait a l'arrangement qui le justifiait.
func TestUneEquipeAppartientAUneOrganisation(t *testing.T) {
	equipe := Team{Name: "Data Science", OrganizationID: "  "}
	if err := equipe.Normalise(); !errors.Is(err, ErrOrganizationRequired) {
		t.Fatalf("acceptee sans organisation : %v", err)
	}
}

// Les bornes existent parce qu'un nom est affiche dans des tableaux, des
// lignes d'audit et des rapports : sans limite, l'appelant choisit la largeur
// de tous les ecrans.
func TestLesLongueursSontBornees(t *testing.T) {
	long := Team{Name: strings.Repeat("a", MaxNameLength+1), OrganizationID: "org-1"}
	if err := long.Normalise(); !errors.Is(err, ErrNameTooLong) {
		t.Errorf("nom trop long accepte : %v", err)
	}
	bavard := Team{Name: "ok", OrganizationID: "org-1",
		Description: strings.Repeat("d", MaxDescriptionLength+1)}
	if err := bavard.Normalise(); !errors.Is(err, ErrDescriptionTooLong) {
		t.Errorf("description trop longue acceptee : %v", err)
	}
}

// Les accents comptent pour un caractere : borner sur les octets refuserait
// des noms francais parfaitement raisonnables.
func TestLaBorneCompteDesCaracteresPasDesOctets(t *testing.T) {
	equipe := Team{Name: strings.Repeat("é", MaxNameLength), OrganizationID: "org-1"}
	if err := equipe.Normalise(); err != nil {
		t.Fatalf("%d caracteres accentues refuses : %v", MaxNameLength, err)
	}
}

func TestLesEspacesSontRognes(t *testing.T) {
	equipe := Team{Name: "  P&C  ", Description: "  actuariat  ", OrganizationID: " org-1 "}
	if err := equipe.Normalise(); err != nil {
		t.Fatal(err)
	}
	if equipe.Name != "P&C" || equipe.Description != "actuariat" || equipe.OrganizationID != "org-1" {
		t.Fatalf("rognage incomplet : %+v", equipe)
	}
}
