package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// handlersAvecProjetReglemente monte le strict necessaire pour que la garde
// du public ait quelque chose a trouver : un projet, un dataset classe hds,
// et le lien entre les deux.
func handlersAvecProjetReglemente(t *testing.T) Handlers {
	t.Helper()
	datasets := memory.NewDatasetStore()
	if err := datasets.Create(dataset.Dataset{ID: "d1", Name: "hds-for", Classification: "hds"}); err != nil {
		t.Fatal(err)
	}
	return Handlers{
		datasetStore:         datasets,
		projectResourceStore: projetAvecDatasets{"projet": {"d1"}},
	}
}

// La regle qui compte, et la raison pour laquelle creation et modification
// partagent une fonction : un projet qui attache de la donnee reglementee ne
// sert pas une application publiquement. Appliquee a la creation seulement,
// elle se contournait en publiant en prive puis en ouvrant l'acces - ce que la
// modification rend possible pour la premiere fois.
func TestUnProjetReglementeNePeutPasOuvrirUneAppAuPublic(t *testing.T) {
	h := handlersAvecProjetReglemente(t)
	recorder := httptest.NewRecorder()
	_, _, _, ok := h.resolveAppAccess(recorder, "projet", "app", "public", nil, nil)
	if ok {
		t.Fatal("le public a ete accorde sur un projet qui attache de la donnee reglementee")
	}
	if recorder.Code != http.StatusConflict {
		t.Fatalf("code %d, attendu 409 : %s", recorder.Code, recorder.Body.String())
	}
}

func TestUnModeInconnuEstRefuseEtLaListeEstExigee(t *testing.T) {
	h := Handlers{}
	cas := []struct {
		mode        string
		users, orgs []string
		code        int
		pourquoi    string
	}{
		{"chuchotement", nil, nil, http.StatusBadRequest, "un mode invente"},
		{"organization", nil, nil, http.StatusBadRequest, "le mode organisation sans organisation"},
		{"users", nil, nil, http.StatusBadRequest, "le mode utilisateurs sans utilisateur"},
		// "project" est le mode d'un tableau de bord, decide par son genre.
		// L'accorder a une application ordinaire elargirait ce qui est permis
		// comme effet de bord du partage de cette fonction.
		{"project", nil, nil, http.StatusBadRequest, "le mode d'un tableau de bord sur une application"},
	}
	for _, item := range cas {
		recorder := httptest.NewRecorder()
		if _, _, _, ok := h.resolveAppAccess(recorder, "projet", "app", item.mode, item.users, item.orgs); ok {
			t.Errorf("%s a ete accepte", item.pourquoi)
			continue
		}
		if recorder.Code != item.code {
			t.Errorf("%s : code %d, attendu %d", item.pourquoi, recorder.Code, item.code)
		}
	}
}

// Un tableau de bord lit le projet auquel il appartient : son public est ce
// projet, et en choisir un autre n'a pas de sens plutot que d'etre refuse.
func TestUnTableauDeBordGardeLeModeDeSonProjet(t *testing.T) {
	h := Handlers{}
	recorder := httptest.NewRecorder()
	mode, users, orgs, ok := h.resolveAppAccess(recorder, "projet", "dashboard", "public", []string{"x"}, []string{"y"})
	if !ok {
		t.Fatalf("refuse : %s", recorder.Body.String())
	}
	if mode != "project" || len(users) != 0 || len(orgs) != 0 {
		t.Fatalf("mode=%q users=%v orgs=%v", mode, users, orgs)
	}
}

// Un mode vide est prive, pas une erreur : c'est le defaut le plus ferme.
func TestUnModeVideEstPrive(t *testing.T) {
	h := Handlers{}
	mode, _, _, ok := h.resolveAppAccess(httptest.NewRecorder(), "projet", "app", "  ", nil, nil)
	if !ok || mode != "private" {
		t.Fatalf("mode=%q ok=%v", mode, ok)
	}
}

// projetAvecDatasets ne repond qu'a la question posee ici : ce qu'un projet
// attache. Le reste de l'interface existe parce qu'elle existe, pas parce que
// ce test s'en sert.
type projetAvecDatasets map[string][]string

func (p projetAvecDatasets) ListProjectDatasetIDs(projectID string) ([]string, error) {
	return p[projectID], nil
}
func (p projetAvecDatasets) ListDatasetProjectIDs(string) ([]string, error)    { return nil, nil }
func (p projetAvecDatasets) AttachDataset(string, string) error                { return nil }
func (p projetAvecDatasets) DetachDataset(string, string) error                { return nil }
func (p projetAvecDatasets) AttachRepository(string, string) error             { return nil }
func (p projetAvecDatasets) DetachRepository(string, string) error             { return nil }
func (p projetAvecDatasets) ListProjectRepositoryIDs(string) ([]string, error) { return nil, nil }
func (p projetAvecDatasets) AttachDatasource(string, string) error             { return nil }
func (p projetAvecDatasets) DetachDatasource(string, string) error             { return nil }
func (p projetAvecDatasets) ListProjectDatasourceIDs(string) ([]string, error) { return nil, nil }
func (p projetAvecDatasets) ListDatasourceProjectIDs(string) ([]string, error) { return nil, nil }
func (p projetAvecDatasets) AttachOntology(string, string) error               { return nil }
func (p projetAvecDatasets) DetachOntology(string, string) error               { return nil }
func (p projetAvecDatasets) ListProjectOntologyIDs(string) ([]string, error)   { return nil, nil }
func (p projetAvecDatasets) AttachExtract(string, string) error                { return nil }
func (p projetAvecDatasets) DetachExtract(string, string) error                { return nil }
func (p projetAvecDatasets) ListProjectExtractIDs(string) ([]string, error)    { return nil, nil }
func (p projetAvecDatasets) ListExtractProjectIDs(string) ([]string, error)    { return nil, nil }
func (p projetAvecDatasets) ListOntologyProjectIDs(string) ([]string, error)   { return nil, nil }
