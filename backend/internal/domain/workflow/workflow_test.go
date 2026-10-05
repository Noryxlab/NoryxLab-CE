package workflow

import (
	"errors"
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/agent"
)

func deuxAgentsEtUneApprobation() Definition {
	return New("stef", "p1", "Veille puis analyse", agent.ScheduleHourly, []Step{
		{Kind: KindAgent, Name: "Chercher", Instruction: "Cherche ce qui est nouveau sur le sujet."},
		{Kind: KindApproval, Name: "Relecture", ApproverUserID: "stef"},
		{Kind: KindAgent, Name: "Analyser", Instruction: "Analyse ce que l'etape precedente a trouve."},
	})
}

// Un workflow sans projet, sans etape, ou avec une etape qu'on ne sait pas
// executer ne doit pas pouvoir etre enregistre.
func TestUneDefinitionIncompleteEstRefusee(t *testing.T) {
	cas := map[string]struct {
		def    Definition
		attend error
	}{
		"sans projet":        {New("stef", "", "x", "manual", []Step{{Kind: KindAgent, Instruction: "i"}}), ErrProjectRequired},
		"sans etape":         {New("stef", "p1", "x", "manual", nil), ErrNoSteps},
		"genre inconnu":      {New("stef", "p1", "x", "manual", []Step{{Kind: "action", Instruction: "i"}}), ErrUnknownKind},
		"agent muet":         {New("stef", "p1", "x", "manual", []Step{{Kind: KindAgent}}), ErrNoInstruction},
		"approbation de nul": {New("stef", "p1", "x", "manual", []Step{{Kind: KindApproval}}), ErrNoApprover},
	}
	for nom, c := range cas {
		if err := c.def.Validate(); !errors.Is(err, c.attend) {
			t.Errorf("%s : erreur %v, attendu %v", nom, err, c.attend)
		}
	}
	if err := deuxAgentsEtUneApprobation().Validate(); err != nil {
		t.Fatalf("une definition complete est refusee : %v", err)
	}
}

// Les etapes sont numerotees par leur position, pas par ce qu'on a tape : la
// position est la verite.
func TestLesEtapesSontNumeroteesParPosition(t *testing.T) {
	def := New("stef", "p1", "x", "manual", []Step{
		{Index: 42, Kind: KindAgent, Instruction: "a"},
		{Index: 7, Kind: KindAgent, Instruction: "b"},
	})
	if def.Steps[0].Index != 0 || def.Steps[1].Index != 1 {
		t.Fatalf("indices = %d, %d", def.Steps[0].Index, def.Steps[1].Index)
	}
}

// Une etape ne peut pas se voir accorder ce qu'un agent ne peut pas tenir.
func TestUneEtapeNeDepassePasUnAgent(t *testing.T) {
	def := New("stef", "p1", "x", "manual", []Step{
		{Kind: KindAgent, Instruction: "i", Actions: []string{"restart_app", "delete_project", "spend_money"}},
	})
	if len(def.Steps[0].Actions) != 1 || def.Steps[0].Actions[0] != agent.ActionRestartApp {
		t.Fatalf("actions = %v", def.Steps[0].Actions)
	}
}

// Le chemin complet : premier agent, attente d'une personne, second agent qui
// recoit ce que le premier a produit.
func TestUnRunTraverseSesEtapesDansLOrdre(t *testing.T) {
	def := deuxAgentsEtUneApprobation()
	run := NewRun(def)
	if run.Status != StatusPending || len(run.Steps) != 3 {
		t.Fatalf("run initial = %s, %d etapes", run.Status, len(run.Steps))
	}

	// Etape 0 : l'agent cherche.
	index, more := run.Current()
	if !more || index != 0 {
		t.Fatalf("courant = %d, %v", index, more)
	}
	run, err := run.Start(0)
	if err != nil {
		t.Fatal(err)
	}
	if run.Steps[0].Attempts != 1 || run.Status != StatusRunning {
		t.Fatalf("apres Start : tentatives %d, statut %s", run.Steps[0].Attempts, run.Status)
	}
	run = run.Succeed(0, "Trois publications nouvelles sur le sujet.", []string{})

	// Etape 1 : le run se gare, et le contexte est le rapport du premier.
	index, _ = run.Current()
	if index != 1 {
		t.Fatalf("courant = %d, attendu 1", index)
	}
	run = run.Wait(1)
	if run.Status != StatusWaitingApproval {
		t.Fatalf("statut = %s", run.Status)
	}
	if run.Context() != "Trois publications nouvelles sur le sujet." {
		t.Fatalf("contexte = %q", run.Context())
	}

	// Quelqu'un d'autre ne peut pas approuver a la place de la personne nommee.
	if _, err := run.Approve(def, "mayssa"); !errors.Is(err, ErrWrongApprover) {
		t.Fatalf("approbation par un tiers : %v", err)
	}
	run, err = run.Approve(def, "stef")
	if err != nil {
		t.Fatal(err)
	}
	if run.Steps[1].Status != StatusSucceeded || run.Steps[1].ApprovedByUserID != "stef" {
		t.Fatalf("etape 1 = %+v", run.Steps[1])
	}

	// Etape 2 : le second agent, puis la fin.
	index, _ = run.Current()
	if index != 2 {
		t.Fatalf("courant = %d, attendu 2", index)
	}
	run, _ = run.Start(2)
	run = run.Succeed(2, "Analyse faite.", []string{})
	if run.Status != StatusSucceeded || run.FinishedAt == nil {
		t.Fatalf("fin = %s, %v", run.Status, run.FinishedAt)
	}
	if _, more := run.Current(); more {
		t.Fatal("un run termine n'a pas d'etape courante")
	}
	if _, err := run.Start(0); !errors.Is(err, ErrRunFinished) {
		t.Fatalf("redemarrer un run termine : %v", err)
	}
}

// Un refus n'est pas une panne : le run est annule, pas echoue, et la liste
// doit pouvoir les distinguer.
func TestUnRefusAnnuleSansEchouer(t *testing.T) {
	def := deuxAgentsEtUneApprobation()
	run := NewRun(def)
	run, _ = run.Start(0)
	run = run.Succeed(0, "trouve", nil)
	run = run.Wait(1)
	run, err := run.Reject(def, "stef")
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != StatusCancelled || run.Error != "" {
		t.Fatalf("statut = %s, erreur = %q", run.Status, run.Error)
	}
	if !run.IsTerminal() {
		t.Fatal("un run annule est termine")
	}
}

// Une etape qui echoue est retentee, et la cle d'idempotence change a chaque
// tentative : une tentative n'est jamais un rejeu de la precedente.
func TestUneEtapeEstRetenteeAvecUneNouvelleCle(t *testing.T) {
	def := New("stef", "p1", "x", "manual", []Step{{Kind: KindAgent, Instruction: "i"}})
	run := NewRun(def)
	cles := map[string]bool{}
	for tentative := 1; tentative <= maxAttempts; tentative++ {
		run, _ = run.Start(0)
		cle := run.IdempotencyKey(0)
		if cles[cle] {
			t.Fatalf("la cle %q se repete", cle)
		}
		cles[cle] = true
		run = run.Fail(0, errors.New("assistant injoignable"))
		if tentative < maxAttempts {
			if run.Status != StatusRunning || run.Steps[0].Status != StatusPending {
				t.Fatalf("tentative %d : run %s, etape %s - devrait etre retentee", tentative, run.Status, run.Steps[0].Status)
			}
			if index, _ := run.Current(); index != 0 {
				t.Fatalf("tentative %d : le run devrait reprendre a l'etape 0, pas %d", tentative, index)
			}
		}
	}
	if run.Status != StatusFailed || run.Steps[0].Attempts != maxAttempts {
		t.Fatalf("apres %d tentatives : %s, %d", maxAttempts, run.Status, run.Steps[0].Attempts)
	}
	if run.Error == "" {
		t.Fatal("un run echoue doit dire quelle etape et pourquoi")
	}
}

// Un run repris apres un redemarrage reprend a la premiere etape non finie,
// derivee de ce qui a ete persiste - jamais d'une memoire du processus.
func TestUnRunRepriseLaOuIlEnEtait(t *testing.T) {
	def := deuxAgentsEtUneApprobation()
	run := NewRun(def)
	run, _ = run.Start(0)
	run = run.Succeed(0, "ok", nil)
	// Le processus meurt ici ; ce qui suit est ce qu'on relit en base.
	relu := run
	index, more := relu.Current()
	if !more || index != 1 {
		t.Fatalf("reprise a %d, attendu 1", index)
	}
}

// Le calendrier est celui d'un agent, sans surprise.
func TestLeCalendrierEstCeluiDUnAgent(t *testing.T) {
	def := New("stef", "p1", "x", agent.ScheduleHourly, []Step{{Kind: KindAgent, Instruction: "i"}})
	now := time.Now().UTC()
	if !def.DueAt(now) {
		t.Fatal("un workflow horaire jamais lance est du")
	}
	il := now.Add(-30 * time.Minute)
	def.LastRunAt = &il
	if def.DueAt(now) {
		t.Fatal("pas du une demi-heure apres")
	}
	il = now.Add(-61 * time.Minute)
	if !def.DueAt(now) {
		t.Fatal("du une heure apres")
	}
	def.Enabled = false
	if def.DueAt(now) {
		t.Fatal("un workflow en pause n'est jamais du")
	}
	manuel := New("stef", "p1", "x", "n'importe quoi", []Step{{Kind: KindAgent, Instruction: "i"}})
	if manuel.Schedule != agent.ScheduleManual || manuel.DueAt(now) {
		t.Fatalf("un calendrier inconnu devient manuel et n'est jamais du : %s", manuel.Schedule)
	}
}
