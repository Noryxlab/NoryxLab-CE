package handlers

import (
	"strings"
	"testing"
)

// Un job construit l'arbre des extraits, comme un workspace.
//
// C'est le trou qui vidait l'objet de son interet. Un extrait est une liste de
// fichiers gelee : elle existe pour qu'un calcul se rejoue dans deux ans sur
// exactement les memes octets. Or la seule chose qui rejoue un calcul sans
// personne devant - le job - ne pouvait pas en monter. L'extrait ne servait
// qu'en interactif, et "le meme n" se refaisait a la main dans le code.
func TestUnJobMonteLesExtraits(t *testing.T) {
	avec := jobBootstrapScript(nil, nil, extractMount{Manifest: "x"})
	if !strings.Contains(avec, "/extracts") || !strings.Contains(avec, "extracts.b64") {
		t.Fatalf("le script d'un job doit construire l'arbre :\n%s", avec)
	}
	sans := jobBootstrapScript(nil, nil, extractMount{})
	if strings.Contains(sans, "extracts.b64") {
		t.Fatal("sans extrait attache, le script ne doit rien construire")
	}
}

// Et une application aussi : elle sert des resultats a des gens, et les servir
// depuis une selection gelee plutot que depuis un bucket qui a grandi est la
// meme exigence.
func TestUneAppMonteLesExtraits(t *testing.T) {
	avec := appBootstrapScript(9000, []string{"python3"}, nil, extractMount{Manifest: "x"})
	if !strings.Contains(avec, "extracts.b64") {
		t.Fatalf("le script d'une app doit construire l'arbre :\n%s", avec)
	}
}

// Une selection trop grande pour tenir dans un manifeste le dit, au lieu de
// monter un arbre partiel.
//
// Un arbre auquel il manque des fichiers est une autre etude, et il ressemble
// a un arbre complet.
func TestUneSelectionTropGrandeLeDitDansUnJob(t *testing.T) {
	script := jobBootstrapScript(nil, nil, extractMount{Refused: 40123})
	if !strings.Contains(script, "40123") || !strings.Contains(script, "not mounted") {
		t.Fatalf("le refus doit etre dit :\n%s", script)
	}
	if strings.Contains(script, "extracts.b64") {
		t.Fatal("un refus ne doit pas tenter de construire l'arbre")
	}
}

// Ce que le montage retient pour la fiche du run.
//
// Un projet change ses rattachements, un resultat ne change pas. "Quelles
// donnees ont produit ce chiffre" n'a de reponse que si la reponse a ete
// ecrite au demarrage.
func TestLeMontageNommeCeQuilAMonte(t *testing.T) {
	montage := extractMount{Manifest: "x", Names: []string{"anterion-3-sujets"}}
	if len(montage.Names) != 1 || montage.Names[0] != "anterion-3-sujets" {
		t.Fatalf("noms = %v", montage.Names)
	}
	if extractSecretData(montage) == nil {
		t.Fatal("un manifeste non vide doit produire le secret qui le porte")
	}
	if extractSecretData(extractMount{}) != nil {
		t.Fatal("sans manifeste, aucun secret - sinon la charge monte un fichier vide")
	}
}
