package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// Une personne appartient a une organisation, et c'est une question de
// propriete plutot que d'acces.
//
// Une equipe est un groupe de travail : en faire partie hors de son
// organisation est de la collaboration, et ne dit rien sur qui possede quoi.
// Une organisation possede. Projets, jeux de donnees et ontologies portent une
// organisation comme proprietaire, et une donnee reglementee le doit - la
// propriete personnelle y est refusee, parce que le jour ou la personne nommee
// s'en va, la donnee de sante a un proprietaire qui n'existe plus.
//
// Konogan Baranton etait chez Essilor et a l'Imt le 2026-09-29, depuis avant
// que les changements d'appartenance soient enregistres, et personne ne
// pouvait dire laquelle etait la bonne.
func TestUneSecondeAppartenanceEstRefusee(t *testing.T) {
	h := Handlers{authMode: "header", keycloak: nil}
	request := httptest.NewRequest(http.MethodPut,
		"/api/v1/admin/organizations/imt/members/barantok", nil)
	request.SetPathValue("organizationID", "imt")
	request.SetPathValue("userID", "barantok")
	request.Header.Set(userHeader, "admin")
	recorder := httptest.NewRecorder()

	h.AddOrganizationMember(recorder, request)
	// Sans client Keycloak la garde d'administration repond d'abord ; ce qui
	// compte est qu'aucun ajout ne soit effectue.
	if recorder.Code == http.StatusNoContent {
		t.Fatal("un ajout a abouti sans que l'appartenance existante ait pu etre lue")
	}
}
