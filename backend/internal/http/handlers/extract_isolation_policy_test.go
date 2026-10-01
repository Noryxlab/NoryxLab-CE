package handlers

import (
	"os"
	"regexp"
	"strings"
	"testing"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

// Les quatre charges appliquent la meme politique d'isolement.
//
// Elle n'existait que pour le workspace, ce qui faisait de la garantie une
// propriete d'un ecran et non de la plateforme : un job du meme projet montait
// le bucket entier, donc "l'equipe voit la selection et rien d'autre" tenait
// pendant qu'on tapait et cessait des qu'un calcul tournait. Une frontiere
// avec une exception n'est pas une frontiere.
//
// Verifie sur le code plutot que par un runtime simule : ce qui compte est
// qu'aucune des quatre ne retombe sur un montage des datasets quand on a
// demande l'isolement, et c'est une propriete du texte.
func TestLesQuatreChargesIsolentDeLaMemeFacon(t *testing.T) {
	for _, fichier := range []string{"workspaces.go", "jobs.go", "cronjobs.go", "apps.go"} {
		source, err := os.ReadFile(fichier)
		if err != nil {
			t.Fatal(err)
		}
		texte := string(source)
		if !strings.Contains(texte, "prepareExtractIsolation") {
			t.Errorf("%s n'utilise pas la politique commune", fichier)
		}
		// Le point qui compte : les volumes de dataset ne sont ajoutes que
		// dans la branche non isolee.
		if fichier == "workspaces.go" {
			continue // Le workspace les ajoute plus haut, derriere son propre test.
		}
		if !regexp.MustCompile(`(?s)if isolated \{.*?\} else \{\s*\S*volumes = append\(volumes, datasetVolumes\.\.\.\)`).MatchString(texte) {
			t.Errorf("%s doit monter les datasets uniquement hors isolement", fichier)
		}
	}
}

// Isoler vers des extraits qu'on n'a pas est refuse, partout.
//
// Une charge isolee vers rien est une charge sans donnees et sans
// explication : la personne conclut que la plateforme a perdu son etude.
func TestIsolerVersRienEstRefuse(t *testing.T) {
	if !refuseEmptyIsolation(true, extractMount{}) {
		t.Fatal("isoler sans extrait doit etre refuse")
	}
	if refuseEmptyIsolation(true, extractMount{Manifest: "x"}) {
		t.Fatal("isoler avec un extrait doit etre accepte")
	}
	if refuseEmptyIsolation(false, extractMount{}) {
		t.Fatal("sans isolement, l'absence d'extrait n'est pas un probleme")
	}
}

// Le remplisseur ne peut jamais ecrire dans la source.
//
// Quel que soit le role de la personne sur le dataset : ce conteneur existe
// pour copier hors du bucket, et rien de ce qu'il fait ne doit pouvoir y
// revenir.
func TestLeRemplisseurNePeutPasEcrireDansLaSource(t *testing.T) {
	h := Handlers{}
	isolement, err := h.prepareExtractIsolation("wks-1", "harbor/vscode:1", []noryxruntime.PersistentVolumeClaimMount{
		{ClaimName: "dataset-hds-for", MountPath: "/datasets/hds-for"},
	})
	if err != nil {
		t.Fatal(err)
	}
	trouve := false
	for _, vol := range isolement.Filler.Volumes {
		if vol.ClaimName == "dataset-hds-for" {
			trouve = true
			if !vol.ReadOnly {
				t.Fatal("la source doit etre montee en lecture seule dans le remplisseur")
			}
		}
	}
	if !trouve {
		t.Fatal("le remplisseur doit voir la source : c'est lui qui copie")
	}
	if !isolement.Tree.ReadOnly {
		t.Fatal("l'arbre monte dans la charge doit etre en lecture seule")
	}
	if isolement.Tree.SubPath != "trees/wks-1" {
		t.Fatalf("sous-chemin = %q : chaque charge a son arbre, pas le cache entier", isolement.Tree.SubPath)
	}
}
