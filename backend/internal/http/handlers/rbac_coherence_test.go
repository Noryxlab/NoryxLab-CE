package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	projectdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/team"
	workspacedomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Ce fichier tient la frontiere que le modele pose : posseder donne
// l'administration, le droit sur la donnee se donne. Chaque test ci-dessous
// couvre un chemin qui etait casse ou faux le 2026-10-23, et qu'aucun test
// n'avait vu passer.

func annuaireEssilor(t *testing.T, membres ...string) *keycloak.Client {
	t.Helper()
	appartenances := map[string][]keycloak.Organization{}
	for _, membre := range membres {
		appartenances[membre] = []keycloak.Organization{
			{ID: "org-essilor", Name: "Essilor", Alias: "essilor", Enabled: true},
		}
	}
	return fakeKeycloak(t, appartenances)
}

// On peut confier un objet a son organisation.
//
// La fonction qui repondait "ce demandeur est-il dans cette organisation"
// parcourait les sujets de la donnee, et ceux-ci ont perdu les organisations
// le jour ou posseder a cesse de vouloir dire lire. La boucle ne pouvait donc
// plus rien trouver : tout transfert vers une organisation etait refuse a tout
// le monde sauf a un administrateur global - alors qu'un dataset reglemente
// doit appartenir a une organisation.
func TestOnConfieUnObjetASonOrganisation(t *testing.T) {
	h := Handlers{keycloak: annuaireEssilor(t, "chercheuse")}
	chercheuse := auth.Identity{Username: "chercheuse", Subject: "chercheuse"}

	kind, id, status, probleme := h.normaliseOwner("organization", "essilor", chercheuse)
	if status != 0 {
		t.Fatalf("transfert refuse : %d %s", status, probleme)
	}
	if kind != "organization" || id != "org-essilor" {
		t.Fatalf("proprietaire normalise = %s/%s", kind, id)
	}

	// Et jamais vers une organisation ou l'on n'est pas.
	etranger := auth.Identity{Username: "etranger", Subject: "etranger"}
	if _, _, status, _ := h.normaliseOwner("organization", "essilor", etranger); status != http.StatusForbidden {
		t.Fatalf("un etranger a l'organisation a pu lui confier un objet (%d)", status)
	}
}

// L'organisation proprietaire peut monter sa donnee reglementee.
//
// La regle demande deux moities : appartenir a l'organisation proprietaire, et
// administrer le projet ou la donnee est montee. La premiere etait evaluee sur
// les sujets de la donnee, qui ne portent plus d'organisation : la regle etait
// redevenue "administrateur global seulement", sans que le message d'erreur le
// dise.
func TestLOrganisationProprietaireMonteSaDonneeReglementee(t *testing.T) {
	h := Handlers{keycloak: annuaireEssilor(t, "chercheuse")}
	chercheuse := auth.Identity{Username: "chercheuse", Subject: "chercheuse"}
	hds := datasetdomain.Dataset{ID: "d1", Classification: "hds", OwnerType: "organization", OwnerID: "org-essilor"}

	if !entitledToRegulatedDataset(h.datasetAdminSubjects(chercheuse), hds) {
		t.Fatal("un membre de l'organisation proprietaire n'est plus habilite")
	}
	etranger := auth.Identity{Username: "etranger", Subject: "etranger"}
	if entitledToRegulatedDataset(h.datasetAdminSubjects(etranger), hds) {
		t.Fatal("un etranger a l'organisation est habilite")
	}
}

// Un octroi a une organisation s'applique.
//
// Accorder la lecture a une organisation est un acte : quelqu'un l'a decide, et
// l'ecran le propose. La liste des sujets ayant perdu les organisations, cet
// octroi etait accepte, affiche dans la matrice, et sans effet. L'heritage que
// l'on a ferme est celui de la propriete.
func TestUnOctroiAUneOrganisationSApplique(t *testing.T) {
	datasets := memory.NewDatasetStore()
	h := Handlers{datasetStore: datasets, keycloak: annuaireEssilor(t, "nouveau")}
	item := datasetdomain.Dataset{ID: "d1", OwnerType: "user", OwnerID: "stef"}
	if err := datasets.Create(item); err != nil {
		t.Fatal(err)
	}
	nouveau := auth.Identity{Username: "nouveau", Subject: "nouveau"}
	if h.datasetRole(item, nouveau) != "" {
		t.Fatal("personne n'a rien avant l'octroi")
	}
	if err := datasets.SetAccess(datasetdomain.Access{
		DatasetID: "d1", SubjectType: "organization", SubjectID: "org-essilor", Role: "reader",
	}); err != nil {
		t.Fatal(err)
	}
	if role := h.datasetRole(item, nouveau); role != "reader" {
		t.Fatalf("octroi a l'organisation = %q", role)
	}
	// Et il n'emporte pas l'administration : lire n'est pas donner.
	if h.canAdministerDataset(item, nouveau) {
		t.Fatal("un octroi de lecture a donne l'administration")
	}
}

// Un extrait ne se lit pas parce que l'organisation le possede.
//
// extractRole appelait canManageExtract, qui compte les organisations depuis
// qu'elles administrent ce qu'elles possedent. Un seul "if" transformait donc
// "j'administre" en "je lis" : chaque membre de l'organisation proprietaire
// ouvrait la liste des fichiers - des chemins qui nomment des patients.
func TestUnExtraitNeSeLitPasParLOrganisationProprietaire(t *testing.T) {
	ontologies := memory.NewOntologyObjectStore()
	onto := ontologydomain.Ontology{ID: "o1", Name: "SELENA", OwnerType: "user", OwnerID: "stef"}
	if err := ontologies.Create(onto); err != nil {
		t.Fatal(err)
	}
	extraits := memory.NewExtractStore()
	item := extractdomain.Extract{ID: "e1", OntologyID: "o1", Name: "test-inria", OwnerType: "organization", OwnerID: "org-essilor"}
	if err := extraits.Create(item, nil); err != nil {
		t.Fatal(err)
	}
	h := Handlers{
		authMode: "header", extractStore: extraits, ontologyStore: ontologies,
		keycloak: annuaireEssilor(t, "nouveau"),
	}
	nouveau := auth.Identity{Username: "nouveau", Subject: "nouveau"}

	if role := h.extractRole(item, nouveau); role != "" {
		t.Fatalf("role sur le contenu = %q, aucun n'etait attendu", role)
	}
	if !h.canManageExtract(item, nouveau) {
		t.Fatal("l'organisation proprietaire doit administrer son extrait")
	}

	// La liste des fichiers lui est refusee...
	membres := httptest.NewRequest(http.MethodGet, "/api/v1/extracts/e1/members", nil)
	membres.Header.Set(userHeader, "nouveau")
	membres.SetPathValue("extractID", "e1")
	vue := httptest.NewRecorder()
	h.GetExtractMembers(vue, membres)
	if vue.Code != http.StatusNotFound {
		t.Fatalf("la liste des fichiers a ete rendue a qui ne la lit pas (%d)", vue.Code)
	}

	// ... et l'ecran des permissions, qu'il decide, lui est rendu.
	permissions := httptest.NewRequest(http.MethodGet, "/api/v1/extracts/e1/access", nil)
	permissions.Header.Set(userHeader, "nouveau")
	permissions.SetPathValue("extractID", "e1")
	lu := httptest.NewRecorder()
	h.ListExtractAccess(lu, permissions)
	if lu.Code != http.StatusOK {
		t.Fatalf("l'administrateur ne voit pas les permissions qu'il peut changer (%d)", lu.Code)
	}
}

// Revoquer par l'alias supprime bien l'octroi.
//
// L'octroi resout "essilor" - l'alias, qui est ce que l'ecran affiche - en
// identifiant d'organisation ; la revocation lisait le chemin brut. Elle ne
// supprimait donc rien et repondait 204 : on croyait avoir retire un acces.
func TestRevoquerParLAliasSupprimeLOctroi(t *testing.T) {
	datasets := memory.NewDatasetStore()
	item := datasetdomain.Dataset{ID: "d1", OwnerType: "user", OwnerID: "stef"}
	if err := datasets.Create(item); err != nil {
		t.Fatal(err)
	}
	if err := datasets.SetAccess(datasetdomain.Access{
		DatasetID: "d1", SubjectType: "organization", SubjectID: "org-essilor", Role: "reader",
	}); err != nil {
		t.Fatal(err)
	}
	h := Handlers{authMode: "header", datasetStore: datasets, keycloak: annuaireEssilor(t, "stef")}

	requete := httptest.NewRequest(http.MethodDelete, "/api/v1/datasets/d1/access/organization/essilor", nil)
	requete.Header.Set(userHeader, "stef")
	requete.SetPathValue("datasetID", "d1")
	requete.SetPathValue("subjectType", "organization")
	requete.SetPathValue("subjectID", "essilor")
	vue := httptest.NewRecorder()
	h.DeleteDatasetAccess(vue, requete)
	if vue.Code != http.StatusNoContent {
		t.Fatalf("revocation = %d, %s", vue.Code, vue.Body.String())
	}
	if _, found, _ := datasets.GetAccess("d1", "organization", "org-essilor"); found {
		t.Fatal("l'octroi est toujours la apres une revocation qui a repondu 204")
	}
}

// Un projet confie a une equipe a un proprietaire.
//
// Le transfert acceptait les trois types de proprietaire ; la fonction qui lit
// la propriete d'un projet ne connaissait que la personne et l'organisation.
// Un projet confie a une equipe devenait donc orphelin sur le champ, et le
// magasin avait accepte l'ecriture sans broncher.
func TestUnProjetConfieAUneEquipeAUnProprietaire(t *testing.T) {
	equipes := memory.NewTeamStore()
	if err := equipes.Create(team.Team{ID: "equipe-data", OrganizationID: "org-essilor", Name: "Data"}); err != nil {
		t.Fatal(err)
	}
	if err := equipes.AddMember("equipe-data", "membre"); err != nil {
		t.Fatal(err)
	}
	h := Handlers{teamStore: equipes}
	item := projectdomain.Project{ID: "p1", Name: "SELENA", OwnerType: "team", OwnerID: "equipe-data"}
	if !h.projectOwnedBy(item, "membre") {
		t.Fatal("un projet confie a une equipe n'a plus de proprietaire")
	}
	if h.projectOwnedBy(item, "etranger") {
		t.Fatal("un etranger a l'equipe possede le projet")
	}
}

// Un workspace s'ouvre par la personne qui l'a lance.
//
// Les datasets dedans ont ete resolus pour une seule identite, au lancement.
// Le seul role de projet gardait la porte, donc un collegue entrait dans le
// workspace d'un autre et lisait ce qui y avait ete monte pour lui, sans
// detenir aucun droit sur cette donnee. Et une ligne anterieure a ce champ ne
// porte pas de lanceur : elle retombe sur le role de projet, parce que
// personne ne doit perdre sa session sur une migration.
func TestUnWorkspaceSOuvreParSonLanceur(t *testing.T) {
	h := Handlers{}
	sien := workspacedomain.Workspace{ID: "w1", ProjectID: "p1", OwnerUserID: "chercheuse"}
	chercheuse := auth.Identity{Username: "chercheuse", Subject: "chercheuse"}
	collegue := auth.Identity{Username: "collegue", Subject: "collegue"}

	if !h.workspaceBelongsTo(sien, chercheuse) {
		t.Fatal("le lanceur n'entre plus dans son propre workspace")
	}
	if h.workspaceBelongsTo(sien, collegue) {
		t.Fatal("un collegue entre dans le workspace d'un autre")
	}

	ancien := workspacedomain.Workspace{ID: "w2", ProjectID: "p1"}
	if !h.workspaceBelongsTo(ancien, collegue) {
		t.Fatal("une ligne sans lanceur doit retomber sur le role de projet")
	}
}

// Et le lancement dit ce qu'il n'a pas monte.
//
// Le filtre etait un "continue" muet : un projet qui attache trois datasets
// demarrait un workspace avec un seul et ne disait rien. La personne voit un
// repertoire vide, sans moyen de savoir si la donnee manque ou si c'est elle.
func TestLeLancementDitCeQuIlNAPasMonte(t *testing.T) {
	datasets := memory.NewDatasetStore()
	lisible := datasetdomain.Dataset{ID: "d1", Name: "Ouvert", OwnerType: "user", OwnerID: "chercheuse"}
	refuse := datasetdomain.Dataset{ID: "d2", Name: "Essilor-data", OwnerType: "organization", OwnerID: "org-essilor"}
	for _, item := range []datasetdomain.Dataset{lisible, refuse} {
		if err := datasets.Create(item); err != nil {
			t.Fatal(err)
		}
	}
	ressources := memory.NewProjectResourceStore()
	if err := ressources.AttachDataset("p1", "d1"); err != nil {
		t.Fatal(err)
	}
	if err := ressources.AttachDataset("p1", "d2"); err != nil {
		t.Fatal(err)
	}
	h := Handlers{
		datasetStore: datasets, projectResourceStore: ressources,
		keycloak: annuaireEssilor(t, "chercheuse"),
	}
	retenus := h.attachmentsWithheldFrom("p1", auth.Identity{Username: "chercheuse", Subject: "chercheuse"})
	if len(retenus) != 1 || retenus[0] != "essilor-data" {
		t.Fatalf("ce qui n'est pas monte = %v", retenus)
	}
}
