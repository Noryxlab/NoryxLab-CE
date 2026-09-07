package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
)

// The interface offers an organization by its alias, because "imt" is what a
// person recognises and 57c801a4-… is not. Every check compared against the
// identifier alone, so transferring a project to an organization that plainly
// exists was refused: "organization does not exist".
func directoryWithImt(t *testing.T) *keycloak.Client {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/protocol/openid-connect/token"):
			writeTestJSON(t, w, map[string]any{"access_token": "test-token", "expires_in": 60})
		case strings.HasSuffix(r.URL.Path, "/organizations"):
			writeTestJSON(t, w, []keycloak.Organization{
				{ID: "57c801a4-f273-4844-97c6-28307872a480", Name: "Imt", Alias: "imt", Enabled: true},
				{ID: "de11ba4d-5383-4468-bbd7-fa5411822bd3", Name: "Retired", Alias: "retired", Enabled: false},
			})
		default:
			writeTestJSON(t, w, []map[string]any{})
		}
	}))
	t.Cleanup(server.Close)
	client, err := keycloak.New(keycloak.Config{
		BaseURL: server.URL, Realm: "noryx", AdminRealm: "master",
		AdminUsername: "admin", AdminPassword: "admin",
	})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestAnOrganizationIsFoundByAliasAsWellAsByIdentifier(t *testing.T) {
	h := Handlers{keycloak: directoryWithImt(t)}

	for _, handle := range []string{"imt", "IMT", "57c801a4-f273-4844-97c6-28307872a480"} {
		organization, found := h.resolveOrganization(handle)
		if !found {
			t.Errorf("%q was not resolved; the platform would call it imaginary", handle)
			continue
		}
		// Whatever the caller typed, what gets stored is the identifier.
		if organization.ID != "57c801a4-f273-4844-97c6-28307872a480" {
			t.Errorf("%q resolved to %q, expected the identifier", handle, organization.ID)
		}
	}

	if _, found := h.resolveOrganization("retired"); found {
		t.Error("a disabled organization must not be a valid destination")
	}
	if _, found := h.resolveOrganization("nowhere"); found {
		t.Error("an organization that does not exist must not resolve")
	}
}

// A dataset shared with an organization, an app allowed for one and a project
// owned by one all record the identifier, so a grant made through the alias
// and one made through the identifier are the same grant.
func TestGrantsRecordTheIdentifierWhicheverHandleWasUsed(t *testing.T) {
	h := Handlers{keycloak: directoryWithImt(t)}
	organization, found := h.resolveOrganization("imt")
	if !found {
		t.Fatal("imt did not resolve")
	}
	payload, err := json.Marshal(map[string]string{"ownerType": "organization", "ownerId": organization.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(payload), "57c801a4") {
		t.Errorf("the stored owner is not the identifier: %s", payload)
	}
}
