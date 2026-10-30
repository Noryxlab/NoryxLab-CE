package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	appdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/app"
	builddomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/build"
	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	datasourcedomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/datasource"
	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	jobdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/job"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	repositorydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/repository"
	workspacedomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Lire un objet par son identifiant, sur les dix types stockes.
//
// Le refus est « introuvable » et jamais « interdit » : nommer un identifiant
// ne doit pas confirmer qu'il existe. Et chaque porte repond la meme chose que
// sa liste, donc un objet absent de votre catalogue est absent ici.
func TestLireUnObjetParSonIdentifiant(t *testing.T) {
	projets := memory.NewProjectStore()
	if err := projets.Create(project.Project{ID: "p1", Name: "Etude", OwnerType: "user", OwnerID: "alice"}); err != nil {
		t.Fatal(err)
	}
	datasets := memory.NewDatasetStore()
	if err := datasets.Create(datasetdomain.Dataset{ID: "d1", Name: "Donnees", OwnerType: "user", OwnerID: "alice", OwnerUserID: "alice"}); err != nil {
		t.Fatal(err)
	}
	ontologies := memory.NewOntologyObjectStore()
	if err := ontologies.Create(ontologydomain.Ontology{ID: "o1", Name: "Onto", OwnerType: "user", OwnerID: "alice", OwnerUserID: "alice"}); err != nil {
		t.Fatal(err)
	}
	extraits := memory.NewExtractStore()
	if err := extraits.Create(extractdomain.Extract{ID: "e1", Name: "Extrait", OntologyID: "o1", ProjectID: "p1", OwnerType: "user", OwnerID: "alice", OwnerUserID: "alice"}, nil); err != nil {
		t.Fatal(err)
	}
	sources := memory.NewDatasourceStore()
	if err := sources.Create(datasourcedomain.Datasource{ID: "s1", Name: "Base", OwnerType: "user", OwnerID: "alice", OwnerUserID: "alice"}); err != nil {
		t.Fatal(err)
	}
	depots := memory.NewRepositoryStore()
	if err := depots.Create(repositorydomain.Repository{ID: "r1", Name: "Depot", OwnerUserID: "alice"}); err != nil {
		t.Fatal(err)
	}
	apps := memory.NewAppStore()
	if err := apps.Create(appdomain.App{ID: "a1", Name: "App", ProjectID: "p1", Kind: "app"}); err != nil {
		t.Fatal(err)
	}
	jobs := memory.NewJobStore()
	if err := jobs.Create(jobdomain.Job{ID: "j1", Name: "Job", ProjectID: "p1"}); err != nil {
		t.Fatal(err)
	}
	workspaces := memory.NewWorkspaceStore()
	if err := workspaces.Create(workspacedomain.Workspace{ID: "w1", Name: "Poste", ProjectID: "p1"}); err != nil {
		t.Fatal(err)
	}
	builds := memory.NewBuildStore()
	if err := builds.Create(builddomain.Build{ID: "b1", Name: "Build", ProjectID: "p1"}); err != nil {
		t.Fatal(err)
	}
	acces := memory.NewAccessStore()
	acces.SetRole("p1", "alice", access.RoleAdmin)

	h := Handlers{
		authMode: "header", projectStore: projets, accessStore: acces,
		datasetStore: datasets, ontologyStore: ontologies, extractStore: extraits,
		datasourceStore: sources, repositoryStore: depots, appStore: apps,
		jobStore: jobs, workspaceStore: workspaces, buildStore: builds,
		projectResourceStore: memory.NewProjectResourceStore(),
	}

	portes := []struct {
		nom     string
		param   string
		id      string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{"dataset", "datasetID", "d1", h.GetDataset},
		{"ontologie", "ontologyID", "o1", h.GetOntology},
		{"extrait", "extractID", "e1", h.GetExtract},
		{"source", "datasourceID", "s1", h.GetDatasource},
		{"depot", "repositoryID", "r1", h.GetRepository},
		{"projet", "projectID", "p1", h.GetProject},
		{"app", "appID", "a1", h.GetApp},
		{"job", "jobID", "j1", h.GetJob},
		{"workspace", "workspaceID", "w1", h.GetWorkspace},
		{"build", "buildID", "b1", h.GetBuild},
	}

	appel := func(porte struct {
		nom     string
		param   string
		id      string
		handler func(http.ResponseWriter, *http.Request)
	}, qui, id string) *httptest.ResponseRecorder {
		requete := httptest.NewRequest(http.MethodGet, "/api/v1/x/"+id, nil)
		requete.Header.Set(userHeader, qui)
		requete.SetPathValue(porte.param, id)
		vue := httptest.NewRecorder()
		porte.handler(vue, requete)
		return vue
	}

	for _, porte := range portes {
		if vue := appel(porte, "alice", porte.id); vue.Code != http.StatusOK {
			t.Errorf("%s : la proprietaire doit le lire, et recoit %d (%s)", porte.nom, vue.Code, vue.Body.String())
		}
		if vue := appel(porte, "bob", porte.id); vue.Code != http.StatusNotFound {
			t.Errorf("%s : un etranger doit recevoir introuvable, et recoit %d", porte.nom, vue.Code)
		}
		if vue := appel(porte, "alice", "inexistant"); vue.Code != http.StatusNotFound {
			t.Errorf("%s : un identifiant inconnu doit etre introuvable, et donne %d", porte.nom, vue.Code)
		}
	}
}

// Ecriture sur un connecteur veut dire quelque chose : changer ce vers quoi
// il pointe. Lecture non, et la plateforme garde les details d'une base
// qu'elle fait tourner elle-meme.
func TestEcrireSurUnConnecteur(t *testing.T) {
	sources := memory.NewDatasourceStore()
	externe := datasourcedomain.Datasource{ID: "s1", Name: "Base", Type: "postgres", Host: "ancien.invalid", Port: 5432,
		Database: "x", Username: "x", PasswordSecret: "mdp", SSLMode: "disable",
		OwnerType: "user", OwnerID: "alice", OwnerUserID: "alice"}
	if err := sources.Create(externe); err != nil {
		t.Fatal(err)
	}
	interne := externe
	interne.ID, interne.Name, interne.Source = "s2", "Base geree", "internal"
	if err := sources.Create(interne); err != nil {
		t.Fatal(err)
	}
	h := Handlers{authMode: "header", datasourceStore: sources, projectResourceStore: memory.NewProjectResourceStore()}

	modifier := func(qui, id, corps string) *httptest.ResponseRecorder {
		requete := httptest.NewRequest(http.MethodPut, "/api/v1/datasources/"+id, strings.NewReader(corps))
		requete.Header.Set(userHeader, qui)
		requete.SetPathValue("datasourceID", id)
		vue := httptest.NewRecorder()
		h.UpdateDatasource(vue, requete)
		return vue
	}

	// Un etranger : introuvable, pas interdit.
	if vue := modifier("bob", "s1", `{"name":"vole"}`); vue.Code != http.StatusNotFound {
		t.Fatalf("un etranger doit recevoir introuvable, pas %d", vue.Code)
	}

	// Lecture octroyee : il la voit, il ne la change pas.
	now := time.Now().UTC()
	if err := sources.SetAccess(datasourcedomain.Access{DatasourceID: "s1", SubjectType: "user", SubjectID: "bob", Role: "reader", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if vue := modifier("bob", "s1", `{"name":"vole"}`); vue.Code != http.StatusForbidden {
		t.Fatalf("Lecture ne change pas un connecteur : %d", vue.Code)
	}

	// Ecriture octroyee : il change l'hote.
	if err := sources.SetAccess(datasourcedomain.Access{DatasourceID: "s1", SubjectType: "user", SubjectID: "bob", Role: "writer", CreatedAt: now, UpdatedAt: now}); err != nil {
		t.Fatal(err)
	}
	if vue := modifier("bob", "s1", `{"name":"Base","host":"nouveau.invalid"}`); vue.Code != http.StatusOK {
		t.Fatalf("Ecriture change un connecteur : %d (%s)", vue.Code, vue.Body.String())
	}
	apres, _, _ := sources.GetByID("s1")
	if apres.Host != "nouveau.invalid" || apres.Database != "x" {
		t.Fatalf("seul ce qui est demande change : %+v", apres)
	}

	// Une base que la plateforme fait tourner garde ses details.
	if vue := modifier("alice", "s2", `{"name":"Base geree","host":"ailleurs.invalid"}`); vue.Code != http.StatusConflict {
		t.Fatalf("les details d'une base geree ne se changent pas : %d", vue.Code)
	}
	if vue := modifier("alice", "s2", `{"name":"Base rebaptisee"}`); vue.Code != http.StatusOK {
		t.Fatalf("son nom, si : %d (%s)", vue.Code, vue.Body.String())
	}
}
