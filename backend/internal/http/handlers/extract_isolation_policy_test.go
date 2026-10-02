package handlers

import (
	"os"
	"regexp"
	"strings"
	"testing"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

// Les quatre charges montent ce que le projet a attache, et ne demandent rien.
//
// Le lancement posait une question - les datasets, ou seulement les extraits -
// et personne ne la comprenait, celui qui l'a ecrite le premier. Ce n'etait pas
// qu'une maladresse de formulation : repondre "extraits" faisait demarrer un
// conteneur de plus, et deux workspaces isoles lances a neuf secondes d'ecart
// se detruisaient l'un l'autre.
//
// La decision appartient au projet. Un dataset qu'il attache est monte, un
// extrait qu'il attache est monte, et c'est vrai d'un workspace, d'un job,
// d'une application et d'une tache planifiee de la meme facon. La frontiere de
// l'ADR-038 devient une consequence de l'attachement et non un mode : un projet
// qui attache un extrait sans le dataset dont il est tire n'atteint pas le
// bucket, parce que rien ne le monte la ou la charge peut le voir.
//
// Verifie sur le texte, parce que ce qui compte est qu'aucune des quatre ne
// retombe sur une branche.
func TestLesQuatreChargesMontentCeQueLeProjetAAttache(t *testing.T) {
	for _, fichier := range []string{"workspaces.go", "jobs.go", "cronjobs.go", "apps.go"} {
		source, err := os.ReadFile(fichier)
		if err != nil {
			t.Fatal(err)
		}
		texte := string(source)
		// Plus de champ, plus de mode, plus de branche.
		for _, interdit := range []string{"DataAccess", "dataAccess", "isolateToExtracts", "if isolated"} {
			if strings.Contains(texte, interdit) {
				t.Errorf("%s porte encore %q : la question est revenue", fichier, interdit)
			}
		}
		// Et les datasets du projet sont montes, sans condition.
		if !regexp.MustCompile(`volumes = append\(volumes, datasetVolumes\.\.\.\)`).MatchString(texte) {
			t.Errorf("%s ne monte pas les datasets du projet", fichier)
		}
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
