package flow

import (
	"strings"
	"testing"
)

func etapes() []Step {
	// Le cas de Clement, qui est la raison d'etre de ce paquet :
	// geometrie -> maillage -> simulation, et un rendu qui ne depend que du
	// maillage et peut donc tourner a cote de la simulation.
	return []Step{
		{Key: "geometrie", Image: "mech:1"},
		{Key: "maillage", Image: "mech:1", DependsOn: []string{"geometrie"}},
		{Key: "simulation", Image: "mech:1", DependsOn: []string{"maillage"}},
		{Key: "rendu", Image: "mech:1", DependsOn: []string{"maillage"}},
	}
}

func etats(valeurs map[string]string) map[string]string {
	out := map[string]string{}
	for _, step := range etapes() {
		out[step.Key] = StepPending
	}
	for key, value := range valeurs {
		out[key] = value
	}
	return out
}

// Un graphe acceptable passe, et c'est la ligne de base.
func TestUnGrapheSainEstAccepte(t *testing.T) {
	if err := Validate(etapes()); err != nil {
		t.Fatalf("graphe sain refuse : %v", err)
	}
}

// Un cycle est refuse a la declaration, et **nomme**. « Il y a un cycle »
// dans un graphe de trente etapes envoie quelqu'un les lire toutes.
func TestUnCycleEstRefuseEtNomme(t *testing.T) {
	err := Validate([]Step{
		{Key: "a", Image: "x", DependsOn: []string{"c"}},
		{Key: "b", Image: "x", DependsOn: []string{"a"}},
		{Key: "c", Image: "x", DependsOn: []string{"b"}},
		{Key: "isole", Image: "x"},
	})
	if err == nil {
		t.Fatal("cycle accepte")
	}
	for _, attendu := range []string{"a", "b", "c"} {
		if !strings.Contains(err.Error(), attendu) {
			t.Errorf("le cycle ne nomme pas %q : %v", attendu, err)
		}
	}
	if strings.Contains(err.Error(), "isole") {
		t.Errorf("l'etape hors du cycle est nommee a tort : %v", err)
	}
}

// Les autres refus portent tous le nom de l'etape fautive.
func TestLesRefusNommentLEtape(t *testing.T) {
	cas := []struct {
		nom     string
		steps   []Step
		attendu string
	}{
		{"dependance inconnue", []Step{{Key: "a", Image: "x", DependsOn: []string{"fantome"}}}, "fantome"},
		{"cle en double", []Step{{Key: "a", Image: "x"}, {Key: "a", Image: "x"}}, "a"},
		{"sans image", []Step{{Key: "a"}}, "a"},
		{"depend de soi", []Step{{Key: "a", Image: "x", DependsOn: []string{"a"}}}, "a"},
		{"vide", nil, "at least one step"},
	}
	for _, c := range cas {
		err := Validate(c.steps)
		if err == nil {
			t.Errorf("%s : accepte", c.nom)
			continue
		}
		if !strings.Contains(err.Error(), c.attendu) {
			t.Errorf("%s : %v ne contient pas %q", c.nom, err, c.attendu)
		}
	}
}

// Au depart, seule l'etape sans dependance peut partir.
func TestSeuleLEtapeSansDependancePartDabord(t *testing.T) {
	got := Runnable(etapes(), etats(nil), 3, 0)
	if len(got) != 1 || got[0] != "geometrie" {
		t.Fatalf("%v", got)
	}
}

// Deux branches independantes partent ensemble : c'est tout l'interet d'un
// graphe par rapport a une sequence.
func TestLesBranchesIndependantesPartentEnsemble(t *testing.T) {
	got := Runnable(etapes(), etats(map[string]string{
		"geometrie": StepSucceeded, "maillage": StepSucceeded,
	}), 3, 0)
	if len(got) != 2 {
		t.Fatalf("%v", got)
	}
}

// Le plafond de parallelisme est respecte, et il compte ce qui tourne deja.
func TestLePlafondCompteCeQuiTourneDeja(t *testing.T) {
	etat := etats(map[string]string{"geometrie": StepSucceeded, "maillage": StepSucceeded})
	if got := Runnable(etapes(), etat, 1, 0); len(got) != 1 {
		t.Fatalf("plafond 1 : %v", got)
	}
	if got := Runnable(etapes(), etat, 2, 2); len(got) != 0 {
		t.Fatalf("deux tournent deja sur un plafond de deux : %v", got)
	}
}

// Une etape reutilisee satisfait ses dependants : pour celui d'apres, la
// sortie existe, qu'elle ait ete produite maintenant ou la derniere fois.
func TestUneEtapeReutiliseeSatisfaitLaSuite(t *testing.T) {
	got := Runnable(etapes(), etats(map[string]string{
		"geometrie": StepReused, "maillage": StepReused,
	}), 3, 0)
	if len(got) != 2 {
		t.Fatalf("%v", got)
	}
}

// Un echec arrete ce qui en depend, **transitivement**, et laisse les
// branches independantes tranquilles.
func TestUnEchecArreteSaBrancheEtPasLesAutres(t *testing.T) {
	steps := append(etapes(), Step{Key: "rapport", Image: "x"})
	etat := etats(map[string]string{"geometrie": StepSucceeded, "maillage": StepFailed})
	etat["rapport"] = StepPending

	raisons := Propagate(steps, etat)
	if etat["simulation"] != StepSkipped || etat["rendu"] != StepSkipped {
		t.Fatalf("la branche en aval n'est pas arretee : %v", etat)
	}
	if etat["rapport"] != StepPending {
		t.Errorf("une branche independante a ete arretee a tort : %v", etat)
	}
	if !strings.Contains(raisons["simulation"], "maillage") {
		t.Errorf("la raison ne nomme pas l'echec : %q", raisons["simulation"])
	}
}

// Un run qui a saute des etapes se termine en echec. Un run vert avec des
// etapes sautees dedans serait un mensonge dit par une couleur.
func TestUnRunQuiSauteDesEtapesEchoue(t *testing.T) {
	fini, statut := Settled(etapes(), etats(map[string]string{
		"geometrie": StepSucceeded, "maillage": StepFailed,
		"simulation": StepSkipped, "rendu": StepSkipped,
	}))
	if !fini || statut != RunFailed {
		t.Fatalf("fini=%v statut=%s", fini, statut)
	}
}

// Et un run encore en cours n'est pas fini, meme si tout le reste a reussi.
func TestUnRunEnCoursNEstPasFini(t *testing.T) {
	fini, statut := Settled(etapes(), etats(map[string]string{
		"geometrie": StepSucceeded, "maillage": StepSucceeded,
		"simulation": StepRunning, "rendu": StepSucceeded,
	}))
	if fini || statut != RunRunning {
		t.Fatalf("fini=%v statut=%s", fini, statut)
	}
}

// Tout reussi, y compris par reutilisation, se termine en succes.
func TestToutReussiSeTermineEnSucces(t *testing.T) {
	fini, statut := Settled(etapes(), etats(map[string]string{
		"geometrie": StepReused, "maillage": StepSucceeded,
		"simulation": StepSucceeded, "rendu": StepSucceeded,
	}))
	if !fini || statut != RunSucceeded {
		t.Fatalf("fini=%v statut=%s", fini, statut)
	}
}
