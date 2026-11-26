package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
)

// adminRequest builds a request whose identity is already resolved - the
// shape an in-process caller has - so these tests exercise the handler's own
// refusals rather than the identity layer's.
func adminRenameRequest(t *testing.T, organizationID, body string) *http.Request {
	t.Helper()
	request := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/organizations/"+organizationID, strings.NewReader(body))
	request = request.WithContext(auth.WithIdentity(request.Context(), auth.Identity{Username: "stef"}))
	if organizationID != "" {
		request.SetPathValue("organizationID", organizationID)
	}
	return request
}

// Renommer ne touche pas l'alias.
//
// Un proprietaire se resout par identifiant **ou** par alias, donc un alias
// qui bouge peut orpheliner un dataset qui le nommait. Le nom affiche n'est
// pointe par rien. C'est la seule raison pour laquelle le renommage est sur
// pour quelqu'un qui s'est trompe de casse a la creation.
func TestLeRenommageRefuseUnNomVide(t *testing.T) {
	h := Handlers{bootstrapAdminUser: "stef"}
	for _, body := range []string{`{"name":""}`, `{"name":"   "}`, `{}`} {
		request := adminRenameRequest(t, "abc", body)
		recorder := httptest.NewRecorder()
		h.RenameOrganization(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("corps %s accepte avec le code %d ; un nom vide effacerait le nom a l'ecran", body, recorder.Code)
		}
	}
}

func TestLeRenommageExigeUnIdentifiant(t *testing.T) {
	h := Handlers{bootstrapAdminUser: "stef"}
	request := adminRenameRequest(t, "", `{"name":"Compta-est"}`)
	recorder := httptest.NewRecorder()
	h.RenameOrganization(recorder, request)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("code %d sans identifiant", recorder.Code)
	}
}

// Sans annuaire configure, la reponse dit ce qui manque plutot que de tomber.
func TestLeRenommageSansAnnuaireLeDitPlutotQueDePaniquer(t *testing.T) {
	h := Handlers{bootstrapAdminUser: "stef"}
	request := adminRenameRequest(t, "abc", `{"name":"Compta-est"}`)
	recorder := httptest.NewRecorder()
	h.RenameOrganization(recorder, request)
	if recorder.Code != http.StatusNotImplemented {
		t.Fatalf("code %d, attendu 501 : %s", recorder.Code, recorder.Body.String())
	}
}
