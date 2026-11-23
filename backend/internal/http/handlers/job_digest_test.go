package handlers

import (
	"os"
	"strings"
	"testing"
)

// L'empreinte d'un lancement ne se fait pas effacer par une mise a jour de
// statut, et une execution planifiee en recoit une.
//
// Les deux moities du meme sujet, trouve le 2026-11-23. Un job lance par
// l'API resout son empreinte avant de partir (ADR-056) ; les executions
// qu'une planification produit sont creees par Kubernetes, donc personne n'a
// rien resolu et leur image restait une etiquette - sur la charge la plus
// susceptible de tourner des mois sans surveillance. Le pod, lui, sait sur
// quelle empreinte il a tourne.
//
// Lu dans la source : ce qu'il faut empecher est la reapparition de l'ordre
// des affectations, et un double de Kubernetes ne dirait pas laquelle des
// deux a gagne.
func TestLEmpreinteDunLancementNEstJamaisEcraseeParLeVeilleur(t *testing.T) {
	source, err := os.ReadFile("jobs.go")
	if err != nil {
		t.Fatal(err)
	}
	body := string(source)

	// La condition qui protege : on n'ecrit l'empreinte du pod que si
	// l'enregistrement n'en porte pas.
	guard := `item.ImageDigest); digest != "" && strings.TrimSpace(record.ImageDigest) == ""`
	if strings.Count(body, guard) != 2 {
		t.Errorf("the watcher must record the pod's digest only when none was recorded, "+
			"and it must do so on both branches - before and after `record = existing`; found %d guard(s)",
			strings.Count(body, guard))
	}

	// Et l'ordre : `record = existing` remplace tout ce qui precede, donc une
	// affectation placee avant lui seulement serait silencieusement perdue.
	assignment := strings.LastIndex(body, guard)
	reset := strings.Index(body, "record = existing")
	if assignment < reset {
		t.Error("the digest is only recorded before `record = existing`, which discards it")
	}
}

// La colonne ne se laisse pas vider non plus.
//
// C'est la forme exacte de deux pannes du 2026-11-23 : un upsert qui ecrit un
// champ que son auteur ne porte pas. La table des builds s'en protegeait
// deja, celle des jobs non.
func TestLaColonneDeLEmpreinteNeSeLaissePasVider(t *testing.T) {
	source, err := os.ReadFile("../../store/postgres/store.go")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(source),
		"image_digest=CASE WHEN EXCLUDED.image_digest = '' THEN jobs.image_digest ELSE EXCLUDED.image_digest END") {
		t.Error("the jobs upsert can blank a recorded image digest")
	}
}
