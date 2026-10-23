package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/team"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Appartenir a une organisation, sans equipe, ne donne rien.
//
// C'etait l'inverse, et ca donnait le role de PROPRIETAIRE : lire, ecrire,
// supprimer, et distribuer les droits. Un compte cree le 2026-10-23 pour un
// essai, pose dans Essilor et dans aucune equipe, pouvait donc supprimer un
// bucket HDS le jour de son arrivee. L'equipe est l'unite dans laquelle on
// ajoute quelqu'un volontairement ; l'organisation dit ou une personne
// travaille, pas ce qu'elle peut lire.
func TestUneOrganisationSansEquipeNeDonneRien(t *testing.T) {
	equipes := memory.NewTeamStore()
	if err := equipes.Create(team.Team{ID: "equipe-essilor", OrganizationID: "org-essilor", Name: "Recherche"}); err != nil {
		t.Fatal(err)
	}
	if err := equipes.AddMember("equipe-essilor", "ancienne"); err != nil {
		t.Fatal(err)
	}
	h := Handlers{
		teamStore: equipes,
		keycloak: fakeKeycloak(t, map[string][]keycloak.Organization{
			"ancienne": {{ID: "org-essilor", Name: "Essilor", Alias: "essilor", Enabled: true}},
			"nouveau":  {{ID: "org-essilor", Name: "Essilor", Alias: "essilor", Enabled: true}},
		}),
	}
	hds := datasetdomain.Dataset{ID: "d1", Name: "HDS-Essilor", OwnerType: "organization", OwnerID: "org-essilor", Classification: "hds"}

	// Le nouveau venu : dans l'organisation, dans aucune equipe.
	nouveau := auth.Identity{Username: "nouveau", Subject: "nouveau"}
	if role := h.datasetRole(hds, nouveau); role != "" {
		t.Fatalf("un compte sans equipe ne doit rien tenir, role = %q", role)
	}
	if h.canReadDataset(hds, nouveau) {
		t.Fatal("un compte sans equipe ne doit pas lire un dataset de l'organisation")
	}

	// Un membre d'equipe de la meme organisation garde ce qu'il avait.
	ancienne := auth.Identity{Username: "ancienne", Subject: "ancienne"}
	if role := h.datasetRole(hds, ancienne); role != "owner" {
		t.Fatalf("un membre d'equipe garde l'heritage de son organisation, role = %q", role)
	}

	// Et une permission nominative suffit, sans equipe : c'est la le droit
	// explicite que la regle laisse passer.
	datasets := memory.NewDatasetStore()
	if err := datasets.Create(hds); err != nil {
		t.Fatal(err)
	}
	if err := datasets.SetAccess(datasetdomain.Access{DatasetID: hds.ID, SubjectType: "user", SubjectID: "nouveau", Role: "reader"}); err != nil {
		t.Fatal(err)
	}
	h.datasetStore = datasets
	if role := h.datasetRole(hds, nouveau); role != "reader" {
		t.Fatalf("un droit nominatif doit passer, role = %q", role)
	}
}

// Un magasin d'equipes injoignable ne coupe pas la plateforme.
//
// La question "cette personne a-t-elle une equipe" doit etre repondable avant
// d'etre opposable : sans ce garde-fou, une panne du magasin rendrait tout le
// monde sans equipe et couperait tous les acces herites d'un coup.
func TestUnMagasinDEquipesAbsentNeCoupeRien(t *testing.T) {
	h := Handlers{
		teamStore: nil,
		keycloak: fakeKeycloak(t, map[string][]keycloak.Organization{
			"quelquun": {{ID: "org-essilor", Name: "Essilor", Alias: "essilor", Enabled: true}},
		}),
	}
	hds := datasetdomain.Dataset{ID: "d1", Name: "HDS-Essilor", OwnerType: "organization", OwnerID: "org-essilor"}
	if role := h.datasetRole(hds, auth.Identity{Username: "quelquun", Subject: "quelquun"}); role != "owner" {
		t.Fatalf("sans magasin d'equipes, l'heritage reste : role = %q", role)
	}
}
