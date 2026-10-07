package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Un runtime qui ne tient plus aucun pod : tout ce qui est en base est mort.
// L'interface est encastree, comme ailleurs dans ces tests, pour n'ecrire que
// la methode dont ce cas a besoin.
type runtimeVide struct{ noryxruntime.Runner }

func (runtimeVide) ListWorkspaces() ([]noryxruntime.WorkspaceRuntimeInfo, error) {
	return nil, nil
}

// L'echantillonneur reconcilie avant de mesurer.
//
// Les enregistrements qu'il lit sont rafraichis par syncWorkspacesFromRuntime,
// qui ne tournait que quand quelqu'un ouvrait un ecran - alors que la mesure
// tourne toutes les cinq minutes, que quelqu'un regarde ou non. Un workspace
// dont le pod etait mort continuait donc d'etre compte, a son CPU et sa memoire
// pleins, jusqu'a ce qu'un humain charge une page. Sur EMSE : 288 echantillons
// consecutifs, vingt-quatre heures, pour un workspace qui n'existait plus.
func TestLEchantillonneurReconcilieAvantDeMesurer(t *testing.T) {
	projets := memory.NewProjectStore()
	if err := projets.Create(project.Project{ID: "p1", Name: "projet"}); err != nil {
		t.Fatal(err)
	}
	espaces := memory.NewWorkspaceStore()
	if err := espaces.Create(workspace.Workspace{
		ID: "w1", ProjectID: "p1", Status: "running",
		CPU: "2", Memory: "4Gi", PodName: "wks-mort",
	}); err != nil {
		t.Fatal(err)
	}

	h := Handlers{
		projectStore:   projets,
		workspaceStore: espaces,
		usageStore:     memory.NewUsageStore(),
		runtime:        runtimeVide{},
	}

	// Avant la mesure, l'enregistrement se dit en marche et serait facture.
	avant, err := h.projectUsage("p1")
	if err != nil {
		t.Fatal(err)
	}
	if avant.Workspaces != 1 {
		t.Fatalf("avant la reconciliation : %d workspace(s) comptes, attendu 1", avant.Workspaces)
	}

	h.sampleUsage()

	apres, err := h.projectUsage("p1")
	if err != nil {
		t.Fatal(err)
	}
	if apres.Workspaces != 0 || apres.VCPU != 0 {
		t.Fatalf("apres la mesure : %d workspace(s) et %v vCPU comptes, attendu 0 : le pod a disparu du cluster",
			apres.Workspaces, apres.VCPU)
	}
}
