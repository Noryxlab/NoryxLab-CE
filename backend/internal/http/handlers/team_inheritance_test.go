package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/team"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

func avecEquipes(t *testing.T) (Handlers, *memory.TeamStore, *memory.AccessStore) {
	t.Helper()
	equipes := memory.NewTeamStore()
	acces := memory.NewAccessStore()
	if err := equipes.Create(team.Team{ID: "t-pc", OrganizationID: "org-scor", Name: "P&C"}); err != nil {
		t.Fatal(err)
	}
	return Handlers{accessStore: acces, teamStore: equipes}, equipes, acces
}

// Les octrois s'additionnent : quitter une equipe retire ce qu'elle donnait,
// et rien d'autre.
func TestUnOctroiDEquipeSAjouteAuDirect(t *testing.T) {
	h, equipes, acces := avecEquipes(t)
	acces.SetRole("projet-1", "alice", access.RoleViewer)
	_ = equipes.AddMember("t-pc", "alice")
	_ = equipes.SetProjectRole("projet-1", "t-pc", access.RoleEditor)

	role, ok := h.effectiveProjectRole("projet-1", "alice")
	if !ok || role != access.RoleEditor {
		t.Fatalf("role = %q (%v), attendu editor : l'equipe doit elargir", role, ok)
	}

	// Elle quitte l'equipe : il lui reste ce qu'on lui avait donne a elle.
	_ = equipes.RemoveMember("t-pc", "alice")
	role, ok = h.effectiveProjectRole("projet-1", "alice")
	if !ok || role != access.RoleViewer {
		t.Fatalf("role = %q (%v), attendu viewer : le direct ne doit pas disparaitre", role, ok)
	}
}

// Un role personnel faible ne doit pas plafonner un octroi d'equipe plus fort,
// ni l'inverse : ce serait une action d'administrateur avec un effet qu'il n'a
// pas demande et qu'il ne voit pas.
func TestLeDirectNePlafonnePasLEquipe(t *testing.T) {
	h, equipes, acces := avecEquipes(t)
	acces.SetRole("projet-1", "alice", access.RoleAdmin)
	_ = equipes.AddMember("t-pc", "alice")
	_ = equipes.SetProjectRole("projet-1", "t-pc", access.RoleViewer)

	if role, _ := h.effectiveProjectRole("projet-1", "alice"); role != access.RoleAdmin {
		t.Fatalf("role = %q, attendu admin : l'equipe ne doit pas rabaisser", role)
	}
}

// Ne pas etre dans l'equipe ne donne rien, meme si l'equipe a un octroi.
func TestUnNonMembreNObtientRien(t *testing.T) {
	h, equipes, _ := avecEquipes(t)
	_ = equipes.SetProjectRole("projet-1", "t-pc", access.RoleAdmin)

	if role, ok := h.effectiveProjectRole("projet-1", "bob"); ok || role != "" {
		t.Fatalf("role = %q (%v) pour un non-membre", role, ok)
	}
}

// Une plateforme sans equipes doit se comporter exactement comme avant leur
// existence : c'est ce qui rend ce deploiement sur.
func TestSansMagasinDEquipesRienNeChange(t *testing.T) {
	acces := memory.NewAccessStore()
	acces.SetRole("projet-1", "alice", access.RoleEditor)
	h := Handlers{accessStore: acces} // teamStore nil

	role, ok := h.effectiveProjectRole("projet-1", "alice")
	if !ok || role != access.RoleEditor {
		t.Fatalf("role = %q (%v) sans magasin d'equipes", role, ok)
	}
}

// Deux equipes, le plus fort gagne - et il n'y a pas de hierarchie entre les
// sources : c'est une comparaison, pas un ordre.
func TestLePlusFortDeDeuxEquipesGagne(t *testing.T) {
	h, equipes, _ := avecEquipes(t)
	_ = equipes.Create(team.Team{ID: "t-sin", OrganizationID: "org-scor", Name: "Sinistres"})
	_ = equipes.AddMember("t-pc", "alice")
	_ = equipes.AddMember("t-sin", "alice")
	_ = equipes.SetProjectRole("projet-1", "t-pc", access.RoleViewer)
	_ = equipes.SetProjectRole("projet-1", "t-sin", access.RoleEditor)

	if role, _ := h.effectiveProjectRole("projet-1", "alice"); role != access.RoleEditor {
		t.Fatalf("role = %q, attendu editor", role)
	}
}
