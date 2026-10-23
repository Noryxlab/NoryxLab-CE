package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	datasourcedomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/datasource"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/team"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Un connecteur se prete, et son usage suit le proprietaire, pas l'auteur.
//
// C'etait le dernier objet du catalogue sans partage : une equipe travaillant
// sur la meme base en declarait un chacun, meme hote et memes identifiants
// recopies a la main. Et l'injection dans une charge comparait l'auteur, si
// bien qu'un connecteur transfere a une equipe cessait de marcher pour elle
// tout en continuant de marcher pour la personne qui l'avait declare.
func TestUnConnecteurSePreteEtSuitSonProprietaire(t *testing.T) {
	equipes := memory.NewTeamStore()
	if err := equipes.Create(team.Team{ID: "equipe-data", OrganizationID: "org", Name: "Data"}); err != nil {
		t.Fatal(err)
	}
	if err := equipes.AddMember("equipe-data", "membre"); err != nil {
		t.Fatal(err)
	}
	sources := memory.NewDatasourceStore()
	item := datasourcedomain.Datasource{ID: "ds1", Name: "Entrepot", OwnerUserID: "auteur", OwnerType: "team", OwnerID: "equipe-data"}
	if err := sources.Create(item); err != nil {
		t.Fatal(err)
	}
	h := Handlers{authMode: "header", datasourceStore: sources, teamStore: equipes}

	membre := auth.Identity{Username: "membre", Subject: "membre"}
	auteur := auth.Identity{Username: "auteur", Subject: "auteur"}
	invite := auth.Identity{Username: "invite", Subject: "invite"}

	// L'equipe proprietaire l'utilise ; l'auteur, qui n'en est plus
	// proprietaire, ne l'utilise plus.
	if !h.canUseDatasource(item, membre) {
		t.Fatal("l'equipe proprietaire doit pouvoir utiliser le connecteur")
	}
	if h.canUseDatasource(item, auteur) {
		t.Fatal("l'auteur d'un connecteur transfere n'y a plus droit")
	}
	if h.canUseDatasource(item, invite) {
		t.Fatal("un tiers n'a rien")
	}

	// Le proprietaire prete l'usage.
	requete := httptest.NewRequest(http.MethodPut, "/api/v1/datasources/ds1/access/user/invite", nil)
	requete.Header.Set(userHeader, "membre")
	requete.SetPathValue("datasourceID", "ds1")
	requete.SetPathValue("subjectType", "user")
	requete.SetPathValue("subjectID", "invite")
	recorder := httptest.NewRecorder()
	h.SetDatasourceAccess(recorder, requete)
	if recorder.Code != http.StatusOK {
		t.Fatalf("octroi = %d, %s", recorder.Code, recorder.Body.String())
	}
	if !h.canUseDatasource(item, invite) {
		t.Fatal("l'invite doit pouvoir utiliser le connecteur")
	}
	if h.canManageDatasource(item, invite) {
		t.Fatal("utiliser n'est pas administrer")
	}

	// Et on reprend.
	retrait := httptest.NewRequest(http.MethodDelete, "/api/v1/datasources/ds1/access/user/invite", nil)
	retrait.Header.Set(userHeader, "membre")
	retrait.SetPathValue("datasourceID", "ds1")
	retrait.SetPathValue("subjectType", "user")
	retrait.SetPathValue("subjectID", "invite")
	vue := httptest.NewRecorder()
	h.DeleteDatasourceAccess(vue, retrait)
	if vue.Code != http.StatusNoContent {
		t.Fatalf("retrait = %d", vue.Code)
	}
	if h.canUseDatasource(item, invite) {
		t.Fatal("apres retrait, plus rien")
	}
}
