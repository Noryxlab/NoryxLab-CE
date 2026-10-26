package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	organizationdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/organization"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/team"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Posseder n'est pas lire.
//
// Appartenir a une organisation donnait le role de proprietaire sur tout ce
// qu'elle possede : lire, ecrire, supprimer, distribuer les droits. Sur la
// plateforme de production, huit buckets HDS appartiennent a une organisation,
// si bien qu'un compte pose dans Essilor pouvait effacer de la donnee de sante
// le jour de son arrivee. Un compte d'essai cree le 2026-10-23 l'a montre.
//
// Le modele separe desormais les deux questions. Une organisation possede : on
// transfere, on accorde, on supprime en son nom. Le droit sur la donnee, lui,
// se donne - a une personne, a une equipe - et se relit dans la matrice des
// acces.
func TestUneOrganisationPossedeMaisNeLitPas(t *testing.T) {
	equipes := memory.NewTeamStore()
	if err := equipes.Create(team.Team{ID: "equipe-essilor", OrganizationID: "org-essilor", Name: "Recherche"}); err != nil {
		t.Fatal(err)
	}
	if err := equipes.AddMember("equipe-essilor", "chercheuse"); err != nil {
		t.Fatal(err)
	}
	datasets := memory.NewDatasetStore()
	organisations := memory.NewOrganizationStore()
	// L'organisation nomme qui agit pour elle. « chercheuse » l'est, le
	// nouveau venu ne l'est pas : c'est toute la difference entre un fait
	// d'annuaire et un droit.
	if err := organisations.AddAdmin(organizationdomain.Admin{
		OrganizationID: "org-essilor", SubjectType: "user", SubjectID: "chercheuse",
	}); err != nil {
		t.Fatal(err)
	}
	h := Handlers{
		teamStore:         equipes,
		datasetStore:      datasets,
		organizationStore: organisations,
		keycloak: fakeKeycloak(t, map[string][]keycloak.Organization{
			"chercheuse": {{ID: "org-essilor", Name: "Essilor", Alias: "essilor", Enabled: true}},
			"nouveau":    {{ID: "org-essilor", Name: "Essilor", Alias: "essilor", Enabled: true}},
		}),
	}
	// Classification laissee vide : en edition Community les datasets HDS sont
	// masques par une autre regle, qui se teste ailleurs et n'a rien a voir
	// avec celle-ci.
	hds := datasetdomain.Dataset{ID: "d1", Name: "Essilor-data", OwnerType: "organization", OwnerID: "org-essilor"}
	if err := datasets.Create(hds); err != nil {
		t.Fatal(err)
	}

	chercheuse := auth.Identity{Username: "chercheuse", Subject: "chercheuse"}
	nouveau := auth.Identity{Username: "nouveau", Subject: "nouveau"}

	// Personne ne lit par appartenance, equipe ou pas.
	for nom, qui := range map[string]auth.Identity{"avec equipe": chercheuse, "sans equipe": nouveau} {
		if role := h.datasetRole(hds, qui); role != "" {
			t.Fatalf("%s : role sur la donnee = %q, aucun n'etait attendu", nom, role)
		}
		if h.canReadDataset(hds, qui) {
			t.Fatalf("%s : l'appartenance ne doit pas ouvrir la donnee", nom)
		}
	}

	// Et l'appartenance seule n'administre pas davantage : le nouveau venu
	// ne voit meme pas l'objet au catalogue.
	if h.canAdministerDataset(hds, nouveau) {
		t.Fatal("l'appartenance a l'organisation a suffi pour administrer")
	}
	if h.canSeeDataset(hds, nouveau) {
		t.Fatal("un membre sans droit voit l'objet de son organisation")
	}

	// Celle que l'organisation a nommee, elle, administre - et ne lit pas
	// pour autant.
	if !h.canAdministerDataset(hds, chercheuse) {
		t.Fatal("l'administratrice nommee par l'organisation n'administre pas")
	}
	if !h.canSeeDataset(hds, chercheuse) {
		t.Fatal("l'objet administre doit rester visible au catalogue")
	}
	if h.canWriteDataset(hds, chercheuse) {
		t.Fatal("administrer n'est pas ecrire")
	}

	// Un droit nominatif ouvre la donnee, et rien d'autre.
	if err := datasets.SetAccess(datasetdomain.Access{DatasetID: hds.ID, SubjectType: "user", SubjectID: "nouveau", Role: "reader"}); err != nil {
		t.Fatal(err)
	}
	if role := h.datasetRole(hds, nouveau); role != "reader" {
		t.Fatalf("droit nominatif = %q", role)
	}

	// Un droit d'equipe aussi.
	if err := datasets.SetAccess(datasetdomain.Access{DatasetID: hds.ID, SubjectType: "team", SubjectID: "equipe-essilor", Role: "writer"}); err != nil {
		t.Fatal(err)
	}
	if role := h.datasetRole(hds, chercheuse); role != "writer" {
		t.Fatalf("droit d'equipe = %q", role)
	}
	if !h.canWriteDataset(hds, chercheuse) {
		t.Fatal("un droit d'ecriture accorde a l'equipe doit ecrire")
	}
	// Et il n'emporte pas l'administration : on peut ecrire sans pouvoir
	// donner la donnee a quelqu'un d'autre.
	autre := auth.Identity{Username: "autre", Subject: "autre"}
	if h.canAdministerDataset(hds, autre) {
		t.Fatal("un inconnu n'administre rien")
	}
}

// Un dataset possede par une equipe se lit par ses membres : l'equipe est
// l'unite ou l'on ajoute quelqu'un volontairement, donc c'est elle qui porte
// le droit sur la donnee.
func TestUneEquipeProprietaireLitSaDonnee(t *testing.T) {
	equipes := memory.NewTeamStore()
	if err := equipes.Create(team.Team{ID: "equipe-mri", OrganizationID: "org-for", Name: "MRI"}); err != nil {
		t.Fatal(err)
	}
	if err := equipes.AddMember("equipe-mri", "membre"); err != nil {
		t.Fatal(err)
	}
	datasets := memory.NewDatasetStore()
	h := Handlers{teamStore: equipes, datasetStore: datasets}
	item := datasetdomain.Dataset{ID: "d2", Name: "IRM", OwnerType: "team", OwnerID: "equipe-mri"}
	if err := datasets.Create(item); err != nil {
		t.Fatal(err)
	}
	membre := auth.Identity{Username: "membre", Subject: "membre"}
	if role := h.datasetRole(item, membre); role != "owner" {
		t.Fatalf("role du membre de l'equipe proprietaire = %q", role)
	}
	if !h.canReadDataset(item, membre) || !h.canWriteDataset(item, membre) || !h.canAdministerDataset(item, membre) {
		t.Fatal("l'equipe proprietaire lit, ecrit et administre")
	}
	if h.canReadDataset(item, auth.Identity{Username: "dehors", Subject: "dehors"}) {
		t.Fatal("quelqu'un hors de l'equipe ne lit rien")
	}
}
