package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/repository"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Une sonde en echec n'empeche pas d'attacher un depot a un projet.
//
// Le veto avait ete leve a la creation et laisse ici, donc le refus s'etait
// simplement deplace d'un ecran : Samy pouvait ajouter son depot Azure DevOps
// puis ne pouvait pas l'attacher, avec la meme phrase. Un veto retire a un
// endroit et laisse a un autre n'est pas retire.
//
// Attacher un depot injoignable est sans danger : un clone qui echoue au
// lancement est deja annonce par le bootstrap et le workspace demarre sans
// lui. Ce que la personne perd en etant refusee, c'est la possibilite de
// configurer son projet - et c'est elle qui sait si le depot clonera.
func TestUneSondeEnEchecNEmpechePasDAttacher(t *testing.T) {
	// Un hote qui repond 404 : exactement ce qu'un depot prive repond a un
	// lecteur qu'il ne reconnait pas.
	serveur := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "", http.StatusNotFound)
	}))
	t.Cleanup(serveur.Close)

	projets := memory.NewProjectStore()
	projet := project.NewOwned("marie", "Etude", "")
	if err := projets.Create(projet); err != nil {
		t.Fatal(err)
	}
	droits := memory.NewAccessStore()
	droits.SetRole(projet.ID, "marie", access.RoleAdmin)

	depots := memory.NewRepositoryStore()
	depot := repository.New("marie", "mca", serveur.URL+"/org/projet/_git/mca", "main", "", "none", "", "")
	if err := depots.Create(depot); err != nil {
		t.Fatal(err)
	}
	ressources := memory.NewProjectResourceStore()

	h := Handlers{
		projectStore:         projets,
		accessStore:          droits,
		repositoryStore:      depots,
		projectResourceStore: ressources,
	}

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPut, "/api/v1/projects/x/repositories/y", nil)
	r.SetPathValue("projectID", projet.ID)
	r.SetPathValue("repositoryID", depot.ID)
	r = r.WithContext(auth.WithIdentity(r.Context(), auth.Identity{Username: "marie"}))
	h.AttachProjectRepository(w, r)

	if w.Code != http.StatusNoContent {
		t.Fatalf("statut %d, attendu 204 : %s", w.Code, w.Body.String())
	}
	attaches, err := ressources.ListProjectRepositoryIDs(projet.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(attaches) != 1 || attaches[0] != depot.ID {
		t.Fatalf("le depot doit etre attache : %v", attaches)
	}

	// Et l'echec n'est pas perdu : il est enregistre sur le depot, ce que la
	// liste du projet affiche desormais.
	relu, found, err := depots.GetByID(depot.ID)
	if err != nil || !found {
		t.Fatal(err)
	}
	if relu.Reachable {
		t.Error("le depot doit rester marque injoignable")
	}
	if relu.ValidationError == "" {
		t.Error("et porter la raison, sinon personne ne sait quoi corriger")
	}
}
