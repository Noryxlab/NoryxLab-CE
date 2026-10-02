package handlers

import (
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	memorystore "github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Un workspace dont le pod a disparu ne doit pas rester "running".
//
// La reconciliation parcourt les pods que le cluster rend et ecrit une ligne
// pour chacun. Une ligne jamais visitee gardait le statut qu'elle avait au
// dernier passage, indefiniment : EMSE en portait une marquee running dont le
// pod etait parti depuis des heures. C'est le meme mensonge que la lecture de
// phase etait venue corriger - son proprietaire clique et recoit une erreur de
// proxy depuis un ecran qui lui disait que tout allait bien.
func TestUnWorkspaceDisparuNEstPlusAnnonceEnMarche(t *testing.T) {
	magasin := memorystore.NewWorkspaceStore()
	for _, w := range []workspace.Workspace{
		{ID: "vivant", ProjectID: "p1", PodName: "wks-vivant", Status: "running",
			Name: "le mien", CreatedAt: time.Now().Add(-time.Hour)},
		{ID: "disparu", ProjectID: "p1", PodName: "wks-disparu", Status: "running",
			Name: "nom choisi par son proprietaire", CreatedAt: time.Now().Add(-2 * time.Hour)},
		{ID: "deja-arrete", ProjectID: "p1", PodName: "wks-vieux", Status: "stopped"},
	} {
		if err := magasin.Create(w); err != nil {
			t.Fatal(err)
		}
	}

	h := Handlers{workspaceStore: magasin}
	// Le cluster ne rend que le premier.
	h.markVanishedWorkspacesStopped(map[string]struct{}{"vivant": {}})

	attendu := map[string]string{"vivant": "running", "disparu": "stopped", "deja-arrete": "stopped"}
	lignes, err := magasin.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(lignes) != 3 {
		t.Fatalf("%d lignes, attendu 3 : rien ne doit etre supprime", len(lignes))
	}
	for _, ligne := range lignes {
		if ligne.Status != attendu[ligne.ID] {
			t.Fatalf("%s : statut %q, attendu %q", ligne.ID, ligne.Status, attendu[ligne.ID])
		}
		// Et la ligne garde ce que son proprietaire a choisi : marquer n'est
		// pas supprimer, c'est tout l'interet.
		if ligne.ID == "disparu" && ligne.Name != "nom choisi par son proprietaire" {
			t.Fatalf("le nom a ete perdu : %q", ligne.Name)
		}
	}
}

// Et sans magasin, la fonction ne panique pas : le mode sans persistance
// existe et ce chemin est appele a chaque listage.
func TestLaReconciliationSansMagasinNePaniquePas(t *testing.T) {
	h := Handlers{}
	h.markVanishedWorkspacesStopped(map[string]struct{}{})
}
