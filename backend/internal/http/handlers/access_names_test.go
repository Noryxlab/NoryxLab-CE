package handlers

import (
	"testing"

	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/team"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Une ligne de permission montre un nom, pas un identifiant.
//
// Une equipe s'affichait en UUID - c'est pourtant le sujet qu'on accorde le
// plus souvent - et une organisation tantot par son nom, tantot par son alias,
// selon que la ligne venait d'un octroi (qui stocke l'identifiant) ou du
// proprietaire (qui peut porter l'alias). Une liste d'acces illisible est une
// liste que personne ne verifie.
func TestUneLignePermissionMontreUnNom(t *testing.T) {
	equipes := memory.NewTeamStore()
	equipe := team.Team{ID: "equipe-ophtalmo", OrganizationID: "org-for", Name: "Ophtalmologie"}
	if err := equipes.Create(equipe); err != nil {
		t.Fatal(err)
	}
	h := Handlers{
		teamStore: equipes,
		keycloak: fakeKeycloak(t, map[string][]keycloak.Organization{
			"stef": {{ID: "org-for", Name: "Fondation Rothschild", Alias: "for", Enabled: true}},
		}),
	}

	lignes := []datasetdomain.Access{
		{SubjectType: "team", SubjectID: equipe.ID},
		{SubjectType: "organization", SubjectID: "org-for"},
		{SubjectType: "organization", SubjectID: "for"},
		{SubjectType: "user", SubjectID: "stef"},
		{SubjectType: "team", SubjectID: "equipe-disparue"},
	}
	h.nameAccessSubjects(lignes)

	attendu := []string{"Ophtalmologie", "Fondation Rothschild", "Fondation Rothschild", "stef", "equipe-disparue"}
	for index, nom := range attendu {
		if lignes[index].SubjectName != nom {
			t.Errorf("ligne %d = %q, attendu %q", index, lignes[index].SubjectName, nom)
		}
	}
}
