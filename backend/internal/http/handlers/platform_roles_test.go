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

// Les lignes fournies ne se modifient pas, et l'administrateur ne s'attribue
// pas : c'est le role de l'annuaire.
func TestLesLignesFourniesSontVerrouillees(t *testing.T) {
	roles := memory.NewPlatformRoleStore()
	h := Handlers{authMode: "header", platformRoleStore: roles, bootstrapAdminUser: "admin"}
	for _, key := range []string{platformrole.KeyUser, platformrole.KeyAdministrator} {
		requete := httptest.NewRequest(http.MethodPut, "/api/v1/admin/platform-roles/"+key, strings.NewReader(`{"name":"x","capabilities":[]}`))
		requete.Header.Set(userHeader, "admin")
		requete.SetPathValue("roleKey", key)
		vue := httptest.NewRecorder()
		h.UpsertPlatformRole(vue, requete)
		if vue.Code != http.StatusConflict {
			t.Fatalf("la ligne fournie %s a ete modifiee (%d)", key, vue.Code)
		}
	}
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
