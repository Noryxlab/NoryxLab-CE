package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/organization"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/platformrole"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Owner, Write, Read (ADR-051). Le proprietaire a RW et partage ; transferer
// laisse a l'ancien proprietaire un octroi Write ecrit dans la liste, jamais
// un droit implicite ; une organisation qui recoit la propriete donne a ses
// membres ce que sa politique dit, par un octroi a l'organisation ; et une
// organisation qui cede la propriete ne laisse rien, ses membres n'ayant
// jamais rien tenu par elle.
func TestLeTransfertLaisseWriteALAncienProprietaire(t *testing.T) {
	datasets := memory.NewDatasetStore()
	if err := datasets.Create(datasetdomain.Dataset{ID: "d1", Name: "Etude", OwnerType: "user", OwnerID: "alice", OwnerUserID: "alice"}); err != nil {
		t.Fatal(err)
	}
	organizations := memory.NewOrganizationStore()
	if err := organizations.SetPolicy(organization.Policy{OrganizationID: "org-for", MembershipGrant: organization.MembershipGrantReader}); err != nil {
		t.Fatal(err)
	}
	h := Handlers{datasetStore: datasets, organizationStore: organizations}

	// Alice confie le dataset a FOR : elle garde Write, FOR lit par sa politique.
	h.settleOwnership("user", "alice", "organization", "org-for", "writer", h.datasetGrantWriter("d1"))
	if access, found, _ := datasets.GetAccess("d1", "user", "alice"); !found || access.Role != "writer" {
		t.Fatalf("l'ancienne proprietaire doit garder Write, pas %+v (trouve=%v)", access, found)
	}
	if access, found, _ := datasets.GetAccess("d1", "organization", "org-for"); !found || access.Role != "reader" {
		t.Fatalf("FOR dit « lecture » : ses membres lisent par un octroi a l'organisation, pas %+v (trouve=%v)", access, found)
	}

	// FOR le confie a bob : l'organisation ne laisse rien derriere elle.
	before, _ := datasets.ListAccess("d1")
	h.settleOwnership("organization", "org-for", "user", "bob", "writer", h.datasetGrantWriter("d1"))
	after, _ := datasets.ListAccess("d1")
	if len(after) != len(before) {
		t.Fatalf("une organisation qui cede ne laisse aucun octroi : %d octrois avant, %d apres", len(before), len(after))
	}

	// Essilor dit « rien » : la propriete ne donne rien a ses membres.
	h.settleOwnership("user", "bob", "organization", "org-essilor", "writer", h.datasetGrantWriter("d1"))
	if _, found, _ := datasets.GetAccess("d1", "organization", "org-essilor"); found {
		t.Fatal("sans politique, l'organisation proprietaire ne recoit aucun octroi")
	}
	if access, found, _ := datasets.GetAccess("d1", "user", "bob"); !found || access.Role != "writer" {
		t.Fatal("bob, ancien proprietaire, garde Write")
	}
}

// La politique se lit et s'ecrit par qui administre l'organisation, se
// limite a trois reponses, et vaut « rien » tant que rien n'a ete dit.
func TestLaPolitiqueDUneOrganisationSeDecideUneFois(t *testing.T) {
	h := Handlers{authMode: "header", organizationStore: memory.NewOrganizationStore(), bootstrapAdminUser: "admin"}
	lire := func(who string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/organizations/org-for/policy", nil)
		req.Header.Set(userHeader, who)
		req.SetPathValue("organizationID", "org-for")
		rec := httptest.NewRecorder()
		h.GetOrganizationPolicy(rec, req)
		return rec
	}
	if rec := lire("admin"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"membershipGrant":"none"`) {
		t.Fatalf("sans reglage, la politique est « rien » : %d %s", rec.Code, rec.Body.String())
	}
	if rec := lire("alice"); rec.Code != http.StatusForbidden {
		t.Fatalf("une personne qui n'administre pas l'organisation ne lit pas sa politique : %d", rec.Code)
	}
	req := httptest.NewRequest(http.MethodPut, "/api/v1/admin/organizations/org-for/policy", strings.NewReader(`{"membershipGrant":"everything"}`))
	req.Header.Set(userHeader, "admin")
	req.SetPathValue("organizationID", "org-for")
	rec := httptest.NewRecorder()
	h.UpdateOrganizationPolicy(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("une reponse hors des trois doit etre refusee : %d", rec.Code)
	}
	req = httptest.NewRequest(http.MethodPut, "/api/v1/admin/organizations/org-for/policy", strings.NewReader(`{"membershipGrant":"reader"}`))
	req.Header.Set(userHeader, "admin")
	req.SetPathValue("organizationID", "org-for")
	rec = httptest.NewRecorder()
	h.UpdateOrganizationPolicy(rec, req)
	if rec.Code != http.StatusOK || h.membershipGrantFor("org-for") != "reader" {
		t.Fatalf("la politique doit etre ecrite : %d %s", rec.Code, rec.Body.String())
	}
}

// Rattacher une donnee reglementee est une case de la matrice : cochee pour
// l'administrateur, pour aucune autre ligne fournie, et une ligne maison
// peut la porter sans etre administrateur de plateforme.
func TestRattacherDuReglementeEstUneFonction(t *testing.T) {
	roles := memory.NewPlatformRoleStore()
	h := Handlers{platformRoleStore: roles}
	cedric := auth.Identity{Username: "cedric", Subject: "cedric"}
	if h.can(cedric, platformrole.AttachRegulatedData) {
		t.Fatal("la ligne utilisateur ne rattache pas de donnee reglementee")
	}
	if err := roles.UpsertCustom(platformrole.Role{Key: "responsable-donnees", Name: "Responsable données",
		Capabilities: []platformrole.Capability{platformrole.AttachRegulatedData}}); err != nil {
		t.Fatal(err)
	}
	if err := roles.SetAssignment(platformrole.Assignment{UserID: "cedric", RoleKey: "responsable-donnees"}); err != nil {
		t.Fatal(err)
	}
	if !h.can(cedric, platformrole.AttachRegulatedData) {
		t.Fatal("la ligne maison qui coche la case rattache du reglemente")
	}
	admin, _ := platformrole.ShippedByKey(platformrole.KeyAdministrator)
	if !admin.Has(platformrole.AttachRegulatedData) {
		t.Fatal("l'administrateur porte la case")
	}
}
