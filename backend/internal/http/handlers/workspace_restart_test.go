package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Un runtime qui note ce qu'on lui demande de redemarrer.
//
// Il n'implemente que PodOperator : c'est exactement la question posee ici -
// un runtime qui ne sait pas redemarrer doit le dire, pas echouer plus loin.
type runtimeRedemarrage struct {
	noryxruntime.Runner
	redemarres []string
	erreur     error
}

func (r *runtimeRedemarrage) GetPodStatus(string) (noryxruntime.PodStatus, error) {
	return noryxruntime.PodStatus{Phase: "Running"}, nil
}
func (r *runtimeRedemarrage) GetPodLogs(string, int) (string, error) { return "", nil }
func (r *runtimeRedemarrage) RestartPod(name string) error {
	if r.erreur != nil {
		return r.erreur
	}
	r.redemarres = append(r.redemarres, name)
	return nil
}

type erreurIntrouvable struct{}

func (erreurIntrouvable) Error() string { return `pods "wks-1" not found` }

// Le montage d'essai : un projet dont « marie » est administratrice, et un
// workspace dedans.
func montageRedemarrage(t *testing.T, moteur noryxruntime.Runner) (Handlers, workspace.Workspace) {
	t.Helper()
	projets := memory.NewProjectStore()
	projet := project.NewOwned("marie", "Etude", "")
	if err := projets.Create(projet); err != nil {
		t.Fatal(err)
	}
	droits := memory.NewAccessStore()
	droits.SetRole(projet.ID, "marie", access.RoleAdmin)

	ateliers := memory.NewWorkspaceStore()
	atelier := workspace.New("slicer", projet.ID, "segmentation",
		"harbor.local/noryx-slicer:1", "wks-1", "wks-1", "2", "8Gi", "", "jeton")
	if err := ateliers.Create(atelier); err != nil {
		t.Fatal(err)
	}
	return Handlers{
		projectStore:   projets,
		accessStore:    droits,
		workspaceStore: ateliers,
		runtime:        moteur,
	}, atelier
}

func demandeRedemarrage(t *testing.T, h Handlers, workspaceID, qui string) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/workspaces/"+workspaceID+"/restart", nil)
	r.SetPathValue("workspaceID", workspaceID)
	r = r.WithContext(auth.WithIdentity(r.Context(), auth.Identity{Username: qui}))
	h.RestartWorkspace(w, r)
	return w
}

// Le redemarrage remplace le pod du workspace, sous le meme nom.
//
// Le manque que cela comble : les applications, tableaux de bord, endpoints et
// services de donnees avaient tous un redemarrage ; les workspaces avaient
// lancer et supprimer, et rien entre les deux. Une extension Slicer qui ne se
// charge qu'au demarrage - nnInteractive, ScriptEditor - ne pouvait donc pas
// etre rendue fonctionnelle du tout.
func TestLeRedemarrageRemplaceLePodSousLeMemeNom(t *testing.T) {
	moteur := &runtimeRedemarrage{}
	h, atelier := montageRedemarrage(t, moteur)

	w := demandeRedemarrage(t, h, atelier.ID, "marie")
	if w.Code != http.StatusAccepted {
		t.Fatalf("statut %d, attendu 202 : %s", w.Code, w.Body.String())
	}
	if len(moteur.redemarres) != 1 || moteur.redemarres[0] != "wks-1" {
		t.Errorf("le pod du workspace doit etre redemarre : %v", moteur.redemarres)
	}
}

// Qui ne peut pas lancer ne peut pas redemarrer.
//
// Le meme droit que lancer et supprimer : redemarrer n'est ni plus ni moins
// que relancer ce qu'on aurait pu lancer.
func TestLeRedemarrageSuitLeDroitDeLancer(t *testing.T) {
	moteur := &runtimeRedemarrage{}
	h, atelier := montageRedemarrage(t, moteur)

	w := demandeRedemarrage(t, h, atelier.ID, "quelquun-dautre")
	if w.Code == http.StatusAccepted {
		t.Errorf("un non-membre ne doit pas redemarrer : %d", w.Code)
	}
	if len(moteur.redemarres) != 0 {
		t.Error("et le runtime ne doit pas avoir ete touche")
	}
}

// Un workspace inconnu est introuvable, un identifiant vide est refuse - et
// dans les deux cas rien n'atteint le runtime.
func TestLeRedemarrageExigeUnWorkspaceConnu(t *testing.T) {
	for id, attendu := range map[string]int{
		"":        http.StatusBadRequest,
		"inconnu": http.StatusNotFound,
	} {
		moteur := &runtimeRedemarrage{}
		h, _ := montageRedemarrage(t, moteur)
		if w := demandeRedemarrage(t, h, id, "marie"); w.Code != attendu {
			t.Errorf("workspaceID %q : statut %d, attendu %d", id, w.Code, attendu)
		}
		if len(moteur.redemarres) != 0 {
			t.Errorf("workspaceID %q a atteint le runtime", id)
		}
	}
}

// Un pod disparu demande un relancement, pas une enquete.
//
// « Le redemarrage a echoue » sur un workspace dont le pod n'existe plus
// envoie quelqu'un chercher une panne qui n'existe pas. Il n'y a rien dont
// relire une specification, et en inventer une serait lancer un nouveau
// workspace en l'appelant un redemarrage.
func TestUnPodDisparuDemandeUnRelancement(t *testing.T) {
	h, atelier := montageRedemarrage(t, &runtimeRedemarrage{erreur: erreurIntrouvable{}})
	w := demandeRedemarrage(t, h, atelier.ID, "marie")
	if w.Code != http.StatusConflict {
		t.Fatalf("statut %d, attendu 409 : %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "launch it again") {
		t.Errorf("le message doit dire quoi faire : %s", w.Body.String())
	}
}

// Un runtime qui ne sait pas redemarrer le dit, plutot que d'echouer ailleurs.
func TestUnRuntimeSansRedemarrageLeDit(t *testing.T) {
	h, atelier := montageRedemarrage(t, nil)
	if w := demandeRedemarrage(t, h, atelier.ID, "marie"); w.Code != http.StatusNotImplemented {
		t.Errorf("statut %d, attendu 501", w.Code)
	}
}
