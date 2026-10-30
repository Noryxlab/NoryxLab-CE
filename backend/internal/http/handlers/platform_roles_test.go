package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/platformrole"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// La matrice gouverne la creation (ADR-049).
//
// Trois lignes fournies : l'administrateur agit sur ce qui n'est pas a lui,
// l'utilisateur sur le sien, le consultant consulte. Personne d'attribue veut
// dire utilisateur, ce que tout compte faisait deja. Et une ligne maison coche
// ce qu'elle veut, sans plafond herite.
func TestLaMatriceDecideDeCeQuUnRolePeutCreer(t *testing.T) {
	roles := memory.NewPlatformRoleStore()
	h := Handlers{platformRoleStore: roles}
	personne := auth.Identity{Username: "alice", Subject: "alice"}

	// Rien d'attribue : utilisateur.
	if role := h.platformRoleOf(personne); role.Key != platformrole.KeyUser {
		t.Fatalf("sans attribution, le role est %q et non utilisateur", role.Key)
	}
	if !h.can(personne, platformrole.CreateProject) || h.can(personne, platformrole.RegisterRegulatedDataset) {
		t.Fatal("un utilisateur cree un projet et n'enregistre pas de donnee reglementee")
	}

	// Consultant : plus rien.
	if err := roles.SetAssignment(platformrole.Assignment{UserID: "alice", RoleKey: platformrole.KeyConsultant}); err != nil {
		t.Fatal(err)
	}
	for _, capability := range platformrole.All {
		if h.can(personne, capability) {
			t.Fatalf("un consultant ne doit pouvoir %s", capability)
		}
	}

	// Une ligne maison qui depasse l'utilisateur : le data steward enregistre
	// de la donnee reglementee sans etre administrateur.
	if err := roles.UpsertCustom(platformrole.Role{
		Key: "data-steward", Name: "Data steward",
		Capabilities: []platformrole.Capability{platformrole.CreateDataset, platformrole.RegisterRegulatedDataset},
	}); err != nil {
		t.Fatal(err)
	}
	if err := roles.SetAssignment(platformrole.Assignment{UserID: "alice", RoleKey: "data-steward"}); err != nil {
		t.Fatal(err)
	}
	if !h.can(personne, platformrole.RegisterRegulatedDataset) {
		t.Fatal("la ligne maison doit pouvoir depasser la ligne utilisateur")
	}
	if h.can(personne, platformrole.LaunchWorkspace) {
		t.Fatal("elle ne porte que ce qu'elle a coche")
	}

	// Une ligne maison supprimee sous ses detenteurs retombe sur
	// utilisateur, jamais sur rien.
	if err := roles.DeleteCustom("data-steward"); err != nil {
		t.Fatal(err)
	}
	if role := h.platformRoleOf(personne); role.Key != platformrole.KeyUser {
		t.Fatalf("un role disparu doit retomber sur utilisateur, pas %q", role.Key)
	}
}

// Le refus nomme le role et la fonction, pour qu'on sache quoi demander.
func TestUnRefusDeLaMatriceDitQuoiDemander(t *testing.T) {
	roles := memory.NewPlatformRoleStore()
	if err := roles.SetAssignment(platformrole.Assignment{UserID: "bob", RoleKey: platformrole.KeyConsultant}); err != nil {
		t.Fatal(err)
	}
	h := Handlers{authMode: "header", platformRoleStore: roles, projectStore: memory.NewProjectStore()}
	requete := httptest.NewRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(`{"name":"essai"}`))
	requete.Header.Set(userHeader, "bob")
	vue := httptest.NewRecorder()
	h.CreateProject(vue, requete)
	if vue.Code != http.StatusForbidden {
		t.Fatalf("un consultant a cree un projet (%d) : %s", vue.Code, vue.Body.String())
	}
	var corps map[string]string
	_ = json.Unmarshal(vue.Body.Bytes(), &corps)
	if corps["role"] != platformrole.KeyConsultant || corps["capability"] != string(platformrole.CreateProject) {
		t.Fatalf("le refus ne dit pas quoi demander : %v", corps)
	}
}

// Le nom d'une ligne fournie reste celui de la plateforme : les cases sont a
// l'installation, le libelle non. Et l'administrateur ne s'attribue pas :
// c'est le role de l'annuaire.
func TestLeLibelleDUneLigneFournieResteCeluiDeLaPlateforme(t *testing.T) {
	roles := memory.NewPlatformRoleStore()
	h := Handlers{authMode: "header", platformRoleStore: roles, bootstrapAdminUser: "admin"}
	requete := httptest.NewRequest(http.MethodPut, "/api/v1/admin/platform-roles/user", strings.NewReader(`{"name":"Chercheur","description":"autre chose","capabilities":[]}`))
	requete.Header.Set(userHeader, "admin")
	requete.SetPathValue("roleKey", platformrole.KeyUser)
	vue := httptest.NewRecorder()
	h.UpsertPlatformRole(vue, requete)
	if vue.Code != http.StatusOK {
		t.Fatalf("les cases d'une ligne fournie sont modifiables (%d)", vue.Code)
	}
	vues, _, err := h.allRoles()
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range vues {
		if role.Key != platformrole.KeyUser {
			continue
		}
		fournie, _ := platformrole.ShippedByKey(platformrole.KeyUser)
		if role.Name != fournie.Name || role.Description != fournie.Description {
			t.Fatalf("le libelle a ete reecrit : %q / %q", role.Name, role.Description)
		}
		if len(role.Capabilities) != 0 {
			t.Fatalf("toutes les cases decochees, la ligne ne porte rien : %+v", role.Capabilities)
		}
		if !role.Changed {
			t.Fatal("une ligne qui ne lit plus le defaut de la plateforme doit le dire")
		}
		return
	}
	t.Fatal("la ligne utilisateur a disparu de la matrice")
}

// Nommer un administrateur ecrit dans l'annuaire, jamais dans la table des
// attributions : sans annuaire, c'est refuse et non range quelque part. Et
// le retirer connait trois refus, dont les deux qu'on peut jouer sans
// annuaire : soi-meme, et le compte d'amorcage.
func TestLAdministrateurSeNommeDansLAnnuaire(t *testing.T) {
	roles := memory.NewPlatformRoleStore()
	h := Handlers{authMode: "header", platformRoleStore: roles, bootstrapAdminUser: "admin"}

	requete := httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/alice/platform-role", strings.NewReader(`{"roleKey":"administrator"}`))
	requete.Header.Set(userHeader, "admin")
	requete.SetPathValue("userID", "alice")
	vue := httptest.NewRecorder()
	h.SetUserPlatformRole(vue, requete)
	if vue.Code != http.StatusNotImplemented {
		t.Fatalf("sans annuaire, nommer un administrateur doit etre refuse, pas %d", vue.Code)
	}
	if _, found, _ := roles.GetAssignment("alice"); found {
		t.Fatal("le role administrateur ne doit jamais etre range dans les attributions")
	}

	// L'administrateur d'amorcage ne se retire pas lui-meme, ni par personne.
	requete = httptest.NewRequest(http.MethodDelete, "/api/v1/admin/users/admin/platform-role", nil)
	requete.Header.Set(userHeader, "admin")
	requete.SetPathValue("userID", "admin")
	vue = httptest.NewRecorder()
	h.ClearUserPlatformRole(vue, requete)
	if vue.Code != http.StatusConflict {
		t.Fatalf("retirer son propre role administrateur doit etre refuse, pas %d", vue.Code)
	}
	requete = httptest.NewRequest(http.MethodPut, "/api/v1/admin/users/admin/platform-role", strings.NewReader(`{"roleKey":"consultant"}`))
	requete.Header.Set(userHeader, "admin")
	requete.SetPathValue("userID", "admin")
	vue = httptest.NewRecorder()
	h.SetUserPlatformRole(vue, requete)
	if vue.Code != http.StatusConflict {
		t.Fatalf("retrograder le compte d'amorcage doit etre refuse, pas %d", vue.Code)
	}
	if _, found, _ := roles.GetAssignment("admin"); found {
		t.Fatal("un refus ne doit rien ecrire")
	}

	// Une fiche d'administrateur se lit comme telle, sans ligne stockee.
	requete = httptest.NewRequest(http.MethodGet, "/api/v1/admin/users/admin/platform-role", nil)
	requete.Header.Set(userHeader, "admin")
	requete.SetPathValue("userID", "admin")
	vue = httptest.NewRecorder()
	h.GetUserPlatformRole(vue, requete)
	var reponse struct {
		Role     platformrole.Role `json:"role"`
		Assigned bool              `json:"assigned"`
	}
	if err := json.NewDecoder(vue.Body).Decode(&reponse); err != nil {
		t.Fatal(err)
	}
	if reponse.Role.Key != platformrole.KeyAdministrator || !reponse.Assigned {
		t.Fatalf("la fiche doit dire administrateur, nomme : %+v", reponse)
	}
}

// Une case inconnue n'est pas stockee : ce serait une case que rien ne lit.
func TestUneCaseInconnueEstRefusee(t *testing.T) {
	h := Handlers{authMode: "header", platformRoleStore: memory.NewPlatformRoleStore(), bootstrapAdminUser: "admin"}
	requete := httptest.NewRequest(http.MethodPut, "/api/v1/admin/platform-roles/steward", strings.NewReader(`{"name":"Steward","capabilities":["delete-everything"]}`))
	requete.Header.Set(userHeader, "admin")
	requete.SetPathValue("roleKey", "steward")
	vue := httptest.NewRecorder()
	h.UpsertPlatformRole(vue, requete)
	if vue.Code != http.StatusBadRequest {
		t.Fatalf("une capacite inconnue a ete acceptee (%d)", vue.Code)
	}
}

// Une ligne fournie se coche sur son installation (ADR-049, revu le
// 30/10/2026) : le nom et la description restent ceux de la plateforme, les
// cases sont a l'installation. Essilor veut que ses utilisateurs rattachent
// de la donnee reglementee sans nommer un role pour tout le monde.
func TestUneLigneFournieSeCocheSurSonInstallation(t *testing.T) {
	roles := memory.NewPlatformRoleStore()
	h := Handlers{authMode: "header", platformRoleStore: roles, bootstrapAdminUser: "admin"}
	personne := auth.Identity{Username: "chercheuse", Subject: "chercheuse"}

	if h.can(personne, platformrole.AttachRegulatedData) {
		t.Fatal("par defaut, la ligne utilisateur ne rattache pas de donnee reglementee")
	}

	// L'administrateur coche la case sur la ligne utilisateur.
	voulu := append([]platformrole.Capability(nil), platformrole.AttachRegulatedData)
	shipped, _ := platformrole.ShippedByKey(platformrole.KeyUser)
	voulu = append(voulu, shipped.Capabilities...)
	corps, _ := json.Marshal(map[string]any{"name": "Utilisateur", "capabilities": voulu})
	requete := httptest.NewRequest(http.MethodPut, "/api/v1/admin/platform-roles/user", strings.NewReader(string(corps)))
	requete.Header.Set(userHeader, "admin")
	requete.SetPathValue("roleKey", platformrole.KeyUser)
	vue := httptest.NewRecorder()
	h.UpsertPlatformRole(vue, requete)
	if vue.Code != http.StatusOK {
		t.Fatalf("la ligne utilisateur doit accepter la case (%d) : %s", vue.Code, vue.Body.String())
	}
	if !h.can(personne, platformrole.AttachRegulatedData) {
		t.Fatal("la case cochee doit valoir pour tout compte sans attribution")
	}
	if h.can(personne, platformrole.AdministerOrganization) {
		t.Fatal("cocher une case n'en coche pas d'autres")
	}

	// C'est une difference, pas une copie : une fonction que la plateforme
	// ajoutera plus tard arrive quand meme - ce que le verrou protegeait.
	stockees, err := roles.ListOverrides()
	if err != nil || len(stockees) != 1 {
		t.Fatalf("une seule difference doit etre stockee : %+v %v", stockees, err)
	}
	if len(stockees[0].Added) != 1 || stockees[0].Added[0] != platformrole.AttachRegulatedData || len(stockees[0].Removed) != 0 {
		t.Fatalf("la difference doit etre « +attach-regulated-data » : %+v", stockees[0])
	}
	augmentee := platformrole.Role{Key: platformrole.KeyUser, Capabilities: []platformrole.Capability{platformrole.CreateProject, platformrole.DeclareRepository}}
	if applied := stockees[0].Apply(augmentee); !applied.Has(platformrole.DeclareRepository) || !applied.Has(platformrole.AttachRegulatedData) {
		t.Fatalf("une ligne fournie qui gagne une fonction doit la garder : %+v", applied.Capabilities)
	}

	// Decochee, l'installation revient a la reponse de la plateforme et rien
	// ne reste stocke.
	corps, _ = json.Marshal(map[string]any{"name": "Utilisateur", "capabilities": shipped.Capabilities})
	requete = httptest.NewRequest(http.MethodPut, "/api/v1/admin/platform-roles/user", strings.NewReader(string(corps)))
	requete.Header.Set(userHeader, "admin")
	requete.SetPathValue("roleKey", platformrole.KeyUser)
	vue = httptest.NewRecorder()
	h.UpsertPlatformRole(vue, requete)
	if vue.Code != http.StatusOK {
		t.Fatalf("decocher doit marcher (%d)", vue.Code)
	}
	if h.can(personne, platformrole.AttachRegulatedData) {
		t.Fatal("decochee, la case ne vaut plus")
	}
	if stockees, _ := roles.ListOverrides(); len(stockees) != 0 {
		t.Fatalf("revenue au defaut, l'installation ne garde aucune difference : %+v", stockees)
	}

	// La ligne administrateur ne se touche pas : c'est celle sur laquelle on
	// retombe, et une case decochee la perdrait.
	corps, _ = json.Marshal(map[string]any{"name": "Administrateur", "capabilities": []string{}})
	requete = httptest.NewRequest(http.MethodPut, "/api/v1/admin/platform-roles/administrator", strings.NewReader(string(corps)))
	requete.Header.Set(userHeader, "admin")
	requete.SetPathValue("roleKey", platformrole.KeyAdministrator)
	vue = httptest.NewRecorder()
	h.UpsertPlatformRole(vue, requete)
	if vue.Code != http.StatusConflict {
		t.Fatalf("la ligne administrateur doit etre refusee (%d)", vue.Code)
	}
}
