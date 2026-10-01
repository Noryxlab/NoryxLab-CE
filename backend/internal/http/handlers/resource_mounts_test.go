package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

func montagesPour(t *testing.T) Handlers {
	t.Helper()
	projets := memory.NewProjectStore()
	for _, item := range []project.Project{
		{ID: "p-sien", Name: "Selena", OwnerType: "user", OwnerID: "stef"},
		{ID: "p-autre", Name: "Etude confidentielle", OwnerType: "user", OwnerID: "alice"},
	} {
		if err := projets.Create(item); err != nil {
			t.Fatal(err)
		}
	}
	acces := memory.NewAccessStore()
	acces.SetRole("p-sien", "stef", access.RoleEditor)
	return Handlers{projectStore: projets, accessStore: acces}
}

// Un nom de projet n'est pas public.
//
// Il porte un client, une etude, parfois une personne. Donc on nomme a
// l'appelant les projets dont il est membre, et on compte les autres : "monte
// dans 1 projet que vous ne voyez pas" est une reponse vraie et utile, la ou
// le silence se lirait "monte nulle part" et inviterait a supprimer.
func TestLesProjetsInvisiblesSontComptesPasNommes(t *testing.T) {
	h := montagesPour(t)
	visibles, caches := h.projectsMounting([]string{"p-sien", "p-autre"}, "stef", false)
	if len(visibles) != 1 || visibles[0].Name != "Selena" {
		t.Fatalf("visibles = %+v, attendu le seul projet dont stef est membre", visibles)
	}
	if caches != 1 {
		t.Fatalf("caches = %d, attendu 1", caches)
	}
}

// Un administrateur global les voit tous, puisqu'il peut les ouvrir.
func TestUnAdministrateurVoitTousLesProjets(t *testing.T) {
	h := montagesPour(t)
	visibles, caches := h.projectsMounting([]string{"p-sien", "p-autre"}, "root", true)
	if len(visibles) != 2 || caches != 0 {
		t.Fatalf("visibles = %d, caches = %d, attendu 2 et 0", len(visibles), caches)
	}
	// Tries par nom : la liste s'affiche, elle ne se parcourt pas.
	if visibles[0].Name != "Etude confidentielle" {
		t.Fatalf("premier = %q, la liste doit etre triee par nom", visibles[0].Name)
	}
}

// Un lien vers un projet disparu est compte, pas nomme.
//
// C'est l'orphelin que le validateur signale, et ce n'est pas quelque chose
// qu'une personne peut traiter depuis cet ecran.
func TestUnLienVersUnProjetDisparuEstCompte(t *testing.T) {
	h := montagesPour(t)
	visibles, caches := h.projectsMounting([]string{"p-sien", "p-efface"}, "root", true)
	if len(visibles) != 1 || caches != 1 {
		t.Fatalf("visibles = %d, caches = %d, attendu 1 et 1", len(visibles), caches)
	}
}

// Et aucun montage rend une liste vide, jamais une erreur.
func TestAucunMontageRendUneListeVide(t *testing.T) {
	h := montagesPour(t)
	visibles, caches := h.projectsMounting(nil, "stef", false)
	if len(visibles) != 0 || caches != 0 {
		t.Fatalf("visibles = %d, caches = %d", len(visibles), caches)
	}
}
