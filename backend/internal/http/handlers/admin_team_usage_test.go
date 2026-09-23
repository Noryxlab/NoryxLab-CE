package handlers

import (
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/team"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/usage"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

func avecConsommation(t *testing.T) (Handlers, *memory.TeamStore) {
	t.Helper()
	echantillons := memory.NewUsageStore()
	base := time.Now().UTC().Add(-2 * time.Hour)
	lot := []usage.Sample{}
	for i := 0; i < 12; i++ {
		quand := base.Add(time.Duration(i) * 5 * time.Minute)
		lot = append(lot,
			usage.Sample{ProjectID: "p1", At: quand, VCPU: 4, MemoryGiB: 8},
			usage.Sample{ProjectID: "p2", At: quand, VCPU: 1, MemoryGiB: 2})
	}
	if err := echantillons.Record(lot); err != nil {
		t.Fatal(err)
	}
	equipes := memory.NewTeamStore()
	_ = equipes.Create(team.Team{ID: "t-a", OrganizationID: "org", Name: "Alpha"})
	_ = equipes.Create(team.Team{ID: "t-b", OrganizationID: "org", Name: "Beta"})
	return Handlers{usageStore: echantillons, teamStore: equipes}, equipes
}

// Un projet atteint par deux equipes a ete paye une fois et travaille par les
// deux. Le couper en deux inventerait une precision que personne n'a mesuree ;
// le rapport credite donc chacune en entier et publie le recouvrement.
func TestLeRecouvrementEstPublieEtPasDivise(t *testing.T) {
	h, equipes := avecConsommation(t)
	_ = equipes.SetProjectRole("p1", "t-a", access.RoleEditor)
	_ = equipes.SetProjectRole("p1", "t-b", access.RoleViewer)

	rapport, err := h.buildTeamUsageReport(time.Now().UTC().Add(-3*time.Hour), time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if len(rapport.Rows) != 2 {
		t.Fatalf("%d lignes, attendu 2", len(rapport.Rows))
	}
	for _, ligne := range rapport.Rows {
		if ligne.SharedProjects != 1 {
			t.Errorf("%s : %d projet partage, attendu 1", ligne.TeamName, ligne.SharedProjects)
		}
	}
	if rapport.AttributedVCPUHours <= rapport.PlatformVCPUHours {
		t.Errorf("l'attribue (%.2f) doit depasser le total plateforme (%.2f) quand deux equipes partagent",
			rapport.AttributedVCPUHours, rapport.PlatformVCPUHours)
	}
}

// Ce qu'aucune equipe n'atteint reste visible : un rapport qui ne montre que
// ce qu'il sait attribuer repond en silence a une autre question.
func TestCeQuiNEstAttribueANulleEquipeResteVisible(t *testing.T) {
	h, equipes := avecConsommation(t)
	_ = equipes.SetProjectRole("p1", "t-a", access.RoleEditor)

	rapport, _ := h.buildTeamUsageReport(time.Now().UTC().Add(-3*time.Hour), time.Now().UTC())
	if rapport.UnattributedVCPUHours <= 0 {
		t.Fatalf("p2 n'est atteint par personne : non attribue = %.2f", rapport.UnattributedVCPUHours)
	}
	if len(rapport.Rows) != 1 || rapport.Rows[0].SharedProjects != 0 {
		t.Fatalf("lignes = %+v", rapport.Rows)
	}
}

// Sans equipes configurees, tout est non attribue - la reponse vraie, pas une
// page vide.
func TestSansEquipesToutEstNonAttribue(t *testing.T) {
	h, _ := avecConsommation(t)
	h.teamStore = nil

	rapport, _ := h.buildTeamUsageReport(time.Now().UTC().Add(-3*time.Hour), time.Now().UTC())
	if len(rapport.Rows) != 0 {
		t.Fatalf("%d lignes sans equipes", len(rapport.Rows))
	}
	if rapport.UnattributedVCPUHours != rapport.PlatformVCPUHours || rapport.PlatformVCPUHours <= 0 {
		t.Fatalf("non attribue %.2f contre plateforme %.2f",
			rapport.UnattributedVCPUHours, rapport.PlatformVCPUHours)
	}
}
