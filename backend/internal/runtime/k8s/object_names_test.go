package k8s

import (
	"strings"
	"testing"
)

// Un nom assemble dont une partie manquait n'est pas vide.
//
// C'est tout le defaut : chaque creation verifiait `!= ""`, et
// `record.PodName + "-user-secrets"` sur un enregistrement sans nom de pod
// donne `-user-secrets`, qui passe cette verification et que Kubernetes
// refuse avec sa propre expression reguliere dans le corps de la reponse.
func TestUnNomAssembleSansSaPartieUtileEstRefuse(t *testing.T) {
	err := validateObjectName("secret", ""+"-user-secrets")
	if err == nil {
		t.Fatal("-user-secrets must be refused: it is what an empty pod name assembles into")
	}
	// Et le message doit designer la cause, pas la regle. Quelqu'un qui lit
	// « doit commencer par un caractere alphanumerique » va corriger un nom
	// qu'il n'a pas choisi ; ce qu'il faut lui dire est que la fiche est
	// incomplete.
	if !strings.Contains(err.Error(), "incomplete") {
		t.Errorf("the message must name the cause, got: %v", err)
	}
	if !strings.Contains(err.Error(), "-user-secrets") {
		t.Errorf("the message must quote the offending name, got: %v", err)
	}
}

func TestLesNomsValidesPassent(t *testing.T) {
	for _, name := range []string{
		"app-7f3a2b1c",
		"app-7f3a2b1c-user-secrets",
		"job-19cea42d3517",
		"noryx-ds-61586d23-018a-4c66-b0ff-8c620da4c26c",
		"a",
		"example.com",
	} {
		if err := validateObjectName("pod", name); err != nil {
			t.Errorf("%q must be accepted: %v", name, err)
		}
	}
}

func TestLesNomsInvalidesSontRefuses(t *testing.T) {
	for _, name := range []string{
		"",                       // rien du tout
		"   ",                    // rien d'utile
		"-leading",               // assemble sans sa partie de tete
		".leading",               // idem, avec un point
		"trailing-",              // assemble sans sa partie de queue
		"Majuscules",             // Kubernetes ne les accepte pas
		"sous_tiret",             // ni le tiret bas
		"espace au milieu",       // ni les espaces
		strings.Repeat("a", 254), // trop long
	} {
		if err := validateObjectName("pod", name); err == nil {
			t.Errorf("%q must be refused", name)
		}
	}
}

// La longueur a son propre message : un nom de 300 caracteres echouait avec le
// meme texte illisible qu'un nom mal forme, et les deux se corrigent
// differemment.
func TestLaLongueurEstDiteCommeTelle(t *testing.T) {
	err := validateObjectName("pod", strings.Repeat("a", 300))
	if err == nil {
		t.Fatal("a 300-character name must be refused")
	}
	if !strings.Contains(err.Error(), "253") {
		t.Errorf("the message must give the limit, got: %v", err)
	}
}

// La limite exacte est acceptee : une verification qui se trompe d'un
// caractere refuse un nom que le cluster aurait pris.
func TestLaLimiteExacteEstAcceptee(t *testing.T) {
	if err := validateObjectName("pod", strings.Repeat("a", 253)); err != nil {
		t.Errorf("253 characters is the limit and must be accepted: %v", err)
	}
}
