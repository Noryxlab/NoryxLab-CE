package budget

import "testing"

// Un budget que personne n'a pose ne doit jamais rien declencher. C'est le
// defaut de tous les projets existants : une alerte sur une limite imaginaire
// apprendrait aux gens a ignorer les alertes.
func TestSansBudgetRienNEstFranchi(t *testing.T) {
	if got := Reached(500, 0); got != 0 {
		t.Fatalf("seuil %d franchi sans budget", got)
	}
	if got := Reached(0, 100); got != 0 {
		t.Fatalf("seuil %d franchi sans consommation", got)
	}
}

// On annonce le seuil le plus haut atteint, pas le premier : un projet qui
// passe de 60 a 120 % entre deux mesures doit entendre « depasse », pas
// « bientot ».
func TestOnAnnonceLePlusHautSeuilAtteint(t *testing.T) {
	cas := []struct {
		consomme, budget float64
		attendu          int
	}{
		{74, 100, 0},
		{75, 100, 75},
		{99, 100, 75},
		{100, 100, 100},
		{250, 100, 100},
	}
	for _, c := range cas {
		if got := Reached(c.consomme, c.budget); got != c.attendu {
			t.Errorf("Reached(%v,%v) = %d, attendu %d", c.consomme, c.budget, got, c.attendu)
		}
	}
}

// Deux seuils, et pas plus. Le test fige la decision : chaque seuil ajoute
// est une notification de plus dans une boite que quelqu'un finira par filtrer.
func TestDeuxSeuilsSeulement(t *testing.T) {
	if len(Thresholds) != 2 || Thresholds[0] != 75 || Thresholds[1] != 100 {
		t.Fatalf("seuils = %v", Thresholds)
	}
}
