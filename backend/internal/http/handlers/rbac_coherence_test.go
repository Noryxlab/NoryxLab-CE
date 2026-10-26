package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	appdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/app"
	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	organizationdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/organization"
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
	organisations := memory.NewOrganizationStore()
	if err := organisations.AddAdmin(organizationdomain.Admin{
		OrganizationID: "org-essilor", SubjectType: "user", SubjectID: "chercheuse",
	}); err != nil {
		t.Fatal(err)
	}
	h := Handlers{keycloak: annuaireEssilor(t, "chercheuse"), organizationStore: organisations}
	chercheuse := auth.Identity{Username: "chercheuse", Subject: "chercheuse"}
	hds := datasetdomain.Dataset{ID: "d1", Classification: "hds", OwnerType: "organization", OwnerID: "org-essilor"}

	if !entitledToRegulatedDataset(h.datasetAdminSubjects(chercheuse), hds) {
		t.Fatal("un membre de l'organisation proprietaire n'est plus habilite")
	}
	etranger := auth.Identity{Username: "etranger", Subject: "etranger"}
	if entitledToRegulatedDataset(h.datasetAdminSubjects(etranger), hds) {
		t.Fatal("un etranger a l'organisation est habilite")
	}
	// Et un simple membre non plus : monter de la donnee reglementee dans un
	// projet, c'est la distribuer.
	membre := auth.Identity{Username: "membre", Subject: "membre"}
	if entitledToRegulatedDataset(h.datasetAdminSubjects(membre), hds) {
		t.Fatal("la seule appartenance a habilite a monter de la donnee reglementee")
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
	organisations := memory.NewOrganizationStore()
	if err := organisations.AddAdmin(organizationdomain.Admin{
		OrganizationID: "org-essilor", SubjectType: "user", SubjectID: "nouveau",
	}); err != nil {
		t.Fatal(err)
	}
	h := Handlers{
		authMode: "header", extractStore: extraits, ontologyStore: ontologies,
		keycloak: annuaireEssilor(t, "nouveau"), organizationStore: organisations,
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

// Administrer une organisation est un droit nomme, pas une appartenance.
//
// C'est la demande d'Essilor, et elle corrige une confusion : l'annuaire dit
// ou quelqu'un travaille, il ne dit pas ce qu'il peut faire. Tant que
// l'organisation n'a nomme personne, ce qu'elle possede n'est administre que
// par un administrateur de la plateforme - la direction qui echoue fermee.
func TestLAdministrationDUneOrganisationSeDonne(t *testing.T) {
	organisations := memory.NewOrganizationStore()
	h := Handlers{
		keycloak:          annuaireEssilor(t, "nouveau", "cyril"),
		organizationStore: organisations,
	}
	hds := datasetdomain.Dataset{ID: "d1", OwnerType: "organization", OwnerID: "org-essilor"}
	nouveau := auth.Identity{Username: "nouveau", Subject: "nouveau"}
	cyril := auth.Identity{Username: "cyril", Subject: "cyril"}

	// Personne n'a ete nomme : les deux membres n'administrent rien.
	for nom, qui := range map[string]auth.Identity{"nouveau": nouveau, "cyril": cyril} {
		if h.canAdministerDataset(hds, qui) {
			t.Fatalf("%s administre sans avoir ete nomme", nom)
		}
		if h.canSeeDataset(hds, qui) {
			t.Fatalf("%s voit au catalogue un objet qu'il n'administre pas", nom)
		}
	}

	// L'organisation nomme Cyril.
	if err := organisations.AddAdmin(organizationdomain.Admin{
		OrganizationID: "org-essilor", SubjectType: "user", SubjectID: "cyril",
	}); err != nil {
		t.Fatal(err)
	}
	if !h.canAdministerDataset(hds, cyril) {
		t.Fatal("l'administrateur nomme n'administre pas")
	}
	if h.canReadDataset(hds, cyril) {
		t.Fatal("etre nomme administrateur a ouvert la donnee")
	}
	if h.canAdministerDataset(hds, nouveau) {
		t.Fatal("nommer quelqu'un a donne le droit a tout le monde")
	}
}

// Et un octroi explicite a l'organisation continue de s'appliquer.
//
// Les deux moities du modele tiennent ensemble : l'appartenance ne confere
// rien d'elle-meme, mais elle reste le chemin par lequel un octroi delibere
// atteint les gens. Sans cela, accorder la lecture a une organisation entiere
// - ce que l'ecran propose - ne ferait rien du tout.
func TestUnOctroiALOrganisationAtteintSesMembresSansLesFaireAdministrer(t *testing.T) {
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
	h := Handlers{
		datasetStore:      datasets,
		keycloak:          annuaireEssilor(t, "nouveau"),
		organizationStore: memory.NewOrganizationStore(),
	}
	nouveau := auth.Identity{Username: "nouveau", Subject: "nouveau"}
	if role := h.datasetRole(item, nouveau); role != "reader" {
		t.Fatalf("l'octroi a l'organisation ne s'applique plus : role = %q", role)
	}
	if h.canAdministerDataset(item, nouveau) {
		t.Fatal("un octroi de lecture a confere l'administration")
	}
}

// Un administrateur nomme agit dans son organisation, et nulle part ailleurs.
//
// C'est la premiere tranche d'ADR-048 : jusqu'ici chaque ecran
// d'administration posait une seule question - « administrateur de la
// plateforme ? » - et repondait toute la plateforme ou rien. Une portee est
// une autorite plus etroite, jamais plus large.
func TestUnAdministrateurNommeNeSortPasDeSonOrganisation(t *testing.T) {
	equipes := memory.NewTeamStore()
	if err := equipes.Create(team.Team{ID: "equipe-essilor", OrganizationID: "org-essilor", Name: "MRI"}); err != nil {
		t.Fatal(err)
	}
	if err := equipes.Create(team.Team{ID: "equipe-for", OrganizationID: "org-for", Name: "Etudes"}); err != nil {
		t.Fatal(err)
	}
	organisations := memory.NewOrganizationStore()
	if err := organisations.AddAdmin(organizationdomain.Admin{
		OrganizationID: "org-essilor", SubjectType: "user", SubjectID: "cyril",
	}); err != nil {
		t.Fatal(err)
	}
	h := Handlers{
		authMode: "header", teamStore: equipes, organizationStore: organisations,
		keycloak: annuaireEssilor(t, "cyril"),
	}
	cyril := auth.Identity{Username: "cyril", Subject: "cyril"}

	if !h.administersOrganization(cyril, "org-essilor") {
		t.Fatal("l'administrateur nomme n'administre pas son organisation")
	}
	if h.administersOrganization(cyril, "org-for") {
		t.Fatal("il administre une organisation pour laquelle il n'a pas ete nomme")
	}

	// Et la portee se lit sur l'equipe, jamais sur la requete : nommer une
	// organisation qu'on administre ne doit pas ouvrir l'equipe d'une autre.
	requete := httptest.NewRequest(http.MethodGet, "/api/v1/admin/teams/equipe-for/members", nil)
	requete.Header.Set(userHeader, "cyril")
	requete.SetPathValue("teamID", "equipe-for")
	vue := httptest.NewRecorder()
	h.ListTeamMembers(vue, requete)
	if vue.Code != http.StatusForbidden {
		t.Fatalf("l'equipe d'une autre organisation a ete rendue (%d)", vue.Code)
	}

	sienne := httptest.NewRequest(http.MethodGet, "/api/v1/admin/teams/equipe-essilor/members", nil)
	sienne.Header.Set(userHeader, "cyril")
	sienne.SetPathValue("teamID", "equipe-essilor")
	lu := httptest.NewRecorder()
	h.ListTeamMembers(lu, sienne)
	if lu.Code != http.StatusOK {
		t.Fatalf("sa propre equipe lui est refusee (%d) : %s", lu.Code, lu.Body.String())
	}
}

// Le lancement nomme ce que la session a recu.
//
// Ce qui est lu a l'interieur d'un workspace ne passe jamais par cette API :
// le bucket est monte par le pilote CSI et geesefs parle a l'objet
// directement. Le journal ne peut donc pas dire ce qui a ete lu, mais il doit
// dire ce qui etait montable ce jour-la - sinon, six mois plus tard, plus
// personne ne peut repondre a « cette personne avait-elle acces a cette etude
// a ce moment-la », alors que les droits ont change depuis.
func TestLeLancementNommeCeQueLaSessionARecu(t *testing.T) {
	montage := []workspaceAttachedDataset{
		{ID: "d1", Name: "HDS-For-selena", Classification: "hds"},
		{ID: "d2", Name: "dataset-local-stef"},
	}
	noms := make([]string, 0, len(montage))
	reglementes := 0
	for _, item := range montage {
		noms = append(noms, item.Name)
		if strings.EqualFold(item.Classification, "hds") {
			reglementes++
		}
	}
	if len(noms) != 2 || reglementes != 1 {
		t.Fatalf("le resume du montage est faux : %v, %d reglemente(s)", noms, reglementes)
	}
	// Et le handler doit bien le poser : la trace serait muette si quelqu'un
	// retirait ces champs du journal de lancement.
	source, err := os.ReadFile("./workspaces.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, champ := range []string{`"datasets":     montes`, `"regulated":    reglementes`, `"withheld":`} {
		if !strings.Contains(string(source), champ) {
			t.Fatalf("le lancement d'un workspace ne journalise plus %s", champ)
		}
	}
}

// Le journal nomme une session sans porter sa cle.
//
// Les evenements de connexion enregistraient le jeton de session comme
// identifiant de ressource, et la table des sessions resout un jeton
// directement : le journal d'audit detenait donc des identifiants utilisables.
// Qui pouvait le lire, l'exporter en CSV ou en restaurer une sauvegarde
// pouvait rejouer une session non expiree. Le DC en portait 178 ; toutes
// avaient expire quand on l'a trouve, ce qui tient de la chance.
func TestLeJournalNePorteJamaisUnJetonDeSession(t *testing.T) {
	jeton := "0f2927d6-708c-4ccf-b7a9-783091eb604c"
	empreinte := sessionFingerprint(jeton)
	if empreinte == "" || strings.Contains(jeton, empreinte) || len(empreinte) != 12 {
		t.Fatalf("empreinte inutilisable : %q", empreinte)
	}
	if sessionFingerprint(jeton) != empreinte {
		t.Fatal("la meme session doit donner la meme empreinte, sinon on ne paire plus une deconnexion a sa connexion")
	}
	if sessionFingerprint("autre-jeton") == empreinte {
		t.Fatal("deux sessions differentes partagent une empreinte")
	}
	// Et le handler ne doit pas revenir au jeton brut.
	source, err := os.ReadFile("./sessions.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, interdit := range []string{`"auth.login", "session", token,`, `"auth.logout", "session", sessionToken,`} {
		if strings.Contains(string(source), interdit) {
			t.Fatalf("le journal porte de nouveau le jeton : %s", interdit)
		}
	}
}

// Chaque role se compte sur sa propre ligne.
//
// Le compte se faisait sur le seul mot du role, ce qui posait tous les
// administrateurs de projet sur la ligne qui decrit l'administration de la
// plateforme : douze administrateurs de projet et six de plateforme se
// lisaient « Administrateur - configuration, gouvernance, audit - porte par
// 12 », et la ligne « Project admin » affichait zero. Un ecran de gouvernance
// qui se trompe sur le role le plus fort est pire qu'un ecran vide.
func TestChaqueRoleSeCompteSurSaLigne(t *testing.T) {
	cas := []struct {
		cellule rbacCell
		attendu string
	}{
		{rbacCell{Role: "admin", ResourceType: "project"}, "project-admin"},
		{rbacCell{Role: "admin", ResourceType: "dataset"}, ""},
		{rbacCell{Role: "owner", ResourceType: "dataset", Source: "owner"}, "owner"},
		{rbacCell{Role: "owner", ResourceType: "dataset", Source: "owner-organization"}, "owner-organization"},
		{rbacCell{Role: "reader", ResourceType: "dataset"}, "reader"},
		{rbacCell{Role: "editor", ResourceType: "project"}, "editor"},
	}
	for _, essai := range cas {
		if obtenu := rbacPolicyRowFor(essai.cellule); obtenu != essai.attendu {
			t.Fatalf("%s sur %s (source %q) -> %q, attendu %q",
				essai.cellule.Role, essai.cellule.ResourceType, essai.cellule.Source, obtenu, essai.attendu)
		}
	}
}

// Donnee reglementee et application publique ne partagent pas un projet.
//
// Une application sert ce que son code veut rendre, a qui son mode d'acces
// admet - et « public » admet l'internet, sans session. Le pod monte les
// datasets du projet, resolus pour la personne qui l'a lancee : une
// application publiee par quelqu'un qui lit un bucket reglemente a ce bucket
// monte derriere une URL qui ne demande rien a personne. Et une vue anonyme
// ne porte aucun acteur, donc le journal ne dirait meme pas qui a regarde.
func TestUneAppPubliqueEtDeLaDonneeReglementeeNeSePartagentPasUnProjet(t *testing.T) {
	datasets := memory.NewDatasetStore()
	hds := datasetdomain.Dataset{ID: "d1", Name: "HDS-For-selena", Classification: "hds", OwnerType: "organization", OwnerID: "org-for"}
	ordinaire := datasetdomain.Dataset{ID: "d2", Name: "meteo", OwnerType: "user", OwnerID: "stef"}
	for _, item := range []datasetdomain.Dataset{hds, ordinaire} {
		if err := datasets.Create(item); err != nil {
			t.Fatal(err)
		}
	}
	ressources := memory.NewProjectResourceStore()
	if err := ressources.AttachDataset("p-hds", "d1"); err != nil {
		t.Fatal(err)
	}
	if err := ressources.AttachDataset("p-ordinaire", "d2"); err != nil {
		t.Fatal(err)
	}
	apps := memory.NewAppStore()
	if err := apps.Upsert(appdomain.App{ID: "a1", ProjectID: "p-public", Name: "meteo-couilly", AccessMode: "public"}); err != nil {
		t.Fatal(err)
	}
	if err := apps.Upsert(appdomain.App{ID: "a2", ProjectID: "p-prive", Name: "interne", AccessMode: "private"}); err != nil {
		t.Fatal(err)
	}
	h := Handlers{datasetStore: datasets, projectResourceStore: ressources, appStore: apps}

	if nom := h.projectHoldsRegulatedDataNamed("p-hds"); nom != "HDS-For-selena" {
		t.Fatalf("la donnee reglementee du projet n'est pas vue : %q", nom)
	}
	if nom := h.projectHoldsRegulatedDataNamed("p-ordinaire"); nom != "" {
		t.Fatalf("un dataset ordinaire est pris pour reglemente : %q", nom)
	}
	if nom := h.projectServesPubliclyNamed("p-public"); nom != "meteo-couilly" {
		t.Fatalf("l'application publique n'est pas vue : %q", nom)
	}
	if nom := h.projectServesPubliclyNamed("p-prive"); nom != "" {
		t.Fatalf("une application privee est prise pour publique : %q", nom)
	}
}

// Une ligne de catalogue ne porte pas le contenu de l'objet.
//
// Administrer met l'objet a l'ecran : il faut bien trouver une chose pour la
// transferer ou decider qui la lit. Ce qui n'etait pas voulu, c'est que la
// ligne porte l'etude elle-meme - une ontologie serialise son manifeste, qui
// nomme chaque sujet avec ses visites et ses modalites, et un extrait
// serialise la liste de sujets qu'il a figee. Les handlers de contenu etaient
// gardes un par un, correctement ; les listes passaient autour.
func TestUneListeNePorteLeContenuQuAQuiPeutLeLire(t *testing.T) {
	ontologies := memory.NewOntologyObjectStore()
	onto := ontologydomain.Ontology{
		ID: "o1", Name: "SELENA", OwnerType: "organization", OwnerID: "org-for",
		Manifest: []byte(`{"subjects":[{"id":"SELENA-0001"}]}`),
	}
	if err := ontologies.Create(onto); err != nil {
		t.Fatal(err)
	}
	organisations := memory.NewOrganizationStore()
	if err := organisations.AddAdmin(organizationdomain.Admin{
		OrganizationID: "org-for", SubjectType: "user", SubjectID: "alice",
	}); err != nil {
		t.Fatal(err)
	}
	h := Handlers{
		ontologyStore:     ontologies,
		organizationStore: organisations,
		keycloak: fakeKeycloak(t, map[string][]keycloak.Organization{
			"alice": {{ID: "org-for", Name: "For", Alias: "for", Enabled: true}},
		}),
	}
	alice := auth.Identity{Username: "alice", Subject: "alice"}

	// Elle administre, donc elle voit la ligne.
	if !h.canAdministerOntology(onto, alice) {
		t.Fatal("l'administratrice nommee doit administrer l'ontologie")
	}
	if h.ontologyRole(onto, alice) != "" {
		t.Fatal("elle ne doit avoir aucun droit sur la donnee")
	}

	// Mais la ligne ne porte plus l'etude.
	items := []ontologydomain.Ontology{onto}
	h.redactOntologiesForReader(items, alice)
	if len(items[0].Manifest) != 0 {
		t.Fatalf("le manifeste est rendu a qui ne lit pas : %s", string(items[0].Manifest))
	}

	// Et il reste entier pour qui en detient la lecture.
	sienne := []ontologydomain.Ontology{{
		ID: "o2", OwnerType: "user", OwnerID: "alice",
		Manifest: []byte(`{"subjects":[{"id":"X"}]}`),
	}}
	h.redactOntologiesForReader(sienne, alice)
	if len(sienne[0].Manifest) == 0 {
		t.Fatal("le manifeste a ete retire a sa proprietaire")
	}
}
