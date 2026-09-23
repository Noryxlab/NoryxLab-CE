package memory

import (
	"errors"
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/team"
)

func equipe(id, org, nom string) team.Team {
	return team.Team{ID: id, OrganizationID: org, Name: nom,
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
}

// Deux clients peuvent chacun avoir une "Data Science" : forcer la difference
// ferait fuiter les locataires d'une installation vers l'autre.
func TestLeNomEstUniqueDansLOrganisationPasGlobalement(t *testing.T) {
	s := NewTeamStore()
	if err := s.Create(equipe("t1", "org-a", "Data Science")); err != nil {
		t.Fatal(err)
	}
	if err := s.Create(equipe("t2", "org-b", "Data Science")); err != nil {
		t.Fatalf("meme nom dans une autre organisation refuse : %v", err)
	}
	if err := s.Create(equipe("t3", "org-a", "data science")); !errors.Is(err, team.ErrNameTaken) {
		t.Fatalf("doublon a la casse pres accepte : %v", err)
	}
}

// Ajouter quelqu'un deja present est normal - un double clic, une liste
// reimportee - et ne doit ni echouer ni deplacer la date d'entree, qui est la
// seule preuve dont dispose un auditeur.
func TestReajouterUnMembreNeDeplacePasSaDateDEntree(t *testing.T) {
	s := NewTeamStore()
	_ = s.Create(equipe("t1", "org-a", "P&C"))
	if err := s.AddMember("t1", "alice"); err != nil {
		t.Fatal(err)
	}
	membres, _ := s.ListMembers("t1")
	premier := membres[0].JoinedAt

	time.Sleep(2 * time.Millisecond)
	if err := s.AddMember("t1", "alice"); err != nil {
		t.Fatalf("reajout refuse : %v", err)
	}
	membres, _ = s.ListMembers("t1")
	if len(membres) != 1 {
		t.Fatalf("%d membres, attendu 1", len(membres))
	}
	if !membres[0].JoinedAt.Equal(premier) {
		t.Fatal("la date d'entree a ete reecrite")
	}
}

// Une equipe supprimee ne doit pas laisser ses octrois derriere elle : une
// permission orpheline est invisible a l'ecran qui l'aurait montree.
func TestSupprimerUneEquipeEmporteSesOctrois(t *testing.T) {
	s := NewTeamStore()
	_ = s.Create(equipe("t1", "org-a", "P&C"))
	_ = s.AddMember("t1", "alice")
	_ = s.SetProjectRole("projet-1", "t1", access.RoleEditor)

	if err := s.Delete("t1"); err != nil {
		t.Fatal(err)
	}
	octrois, _ := s.ListProjectRoles("projet-1")
	if len(octrois) != 0 {
		t.Fatalf("octrois orphelins : %+v", octrois)
	}
	if teams, _ := s.ListByUser("alice"); len(teams) != 0 {
		t.Fatalf("appartenance orpheline : %+v", teams)
	}
}

// Un role vide revoque plutot que de stocker un blanc : une ligne disant
// "aucun role" et pas de ligne du tout sont le meme fait.
func TestUnRoleVideRevoque(t *testing.T) {
	s := NewTeamStore()
	_ = s.Create(equipe("t1", "org-a", "P&C"))
	_ = s.SetProjectRole("projet-1", "t1", access.RoleViewer)
	_ = s.SetProjectRole("projet-1", "t1", "")

	if octrois, _ := s.ListProjectRoles("projet-1"); len(octrois) != 0 {
		t.Fatalf("octroi conserve apres revocation : %+v", octrois)
	}
}

// Ce que la voie d'autorisation demande a chaque requete : les octrois qui
// atteignent cette personne, et eux seuls.
func TestSeulsLesOctroisDeSesEquipesRemontent(t *testing.T) {
	s := NewTeamStore()
	_ = s.Create(equipe("t1", "org-a", "P&C"))
	_ = s.Create(equipe("t2", "org-a", "Sinistres"))
	_ = s.AddMember("t1", "alice")
	_ = s.AddMember("t2", "bob")
	_ = s.SetProjectRole("projet-1", "t1", access.RoleEditor)
	_ = s.SetProjectRole("projet-1", "t2", access.RoleAdmin)

	pour, _ := s.ListProjectRolesForUser("projet-1", "alice")
	if len(pour) != 1 || pour[0].Role != access.RoleEditor {
		t.Fatalf("alice devrait n'avoir que l'octroi de P&C : %+v", pour)
	}
	if pour[0].TeamName != "P&C" {
		t.Errorf("le nom d'equipe n'est pas resolu : %+v", pour[0])
	}
}
