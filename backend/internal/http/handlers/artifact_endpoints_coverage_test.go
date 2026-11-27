package handlers

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// Tout ce qui lance une charge doit lui dire ou sont les paquets.
//
// Le commentaire de workloadBaseEnv l'annonçait : « quatre endroits
// construisent l'environnement d'une charge, et celui qu'on oublie est celui
// qui casse apres la fermeture du reseau - en silence, et seulement pour qui
// lance ce genre de charge la ». Les jobs et les jobs planifies etaient
// l'oubli. Trouve le 27/11/2026 en preparant la fermeture du reseau au DC :
// Pulp repondait, et la charge ne le connaissait pas.
//
// Lu comme du texte plutot que monte : ce qu'il faut verifier est qu'aucun
// site de lancement ne construise son environnement sans passer par le
// helper, et ca ne se voit ni au typage ni a l'execution d'un test unitaire.
func TestChaqueLancementHeriteDesMiroirs(t *testing.T) {
	fichiers := []string{"jobs.go", "cronjobs.go", "apps.go", "workspaces.go"}
	// Un Env construit a la main, sans le helper : la forme exacte de l'oubli.
	suspect := regexp.MustCompile(`Env:\s+append\(datasourceEnv`)

	for _, nom := range fichiers {
		source, err := os.ReadFile(nom)
		if err != nil {
			t.Fatalf("lecture de %s : %v", nom, err)
		}
		corps := string(source)
		if suspect.MatchString(corps) {
			t.Errorf("%s construit l'environnement d'une charge sans workloadBaseEnv : "+
				"cette charge ne connaitra ni l'index pip, ni le canal conda, ni le miroir deb, "+
				"et elle cassera le jour ou le reseau se ferme", nom)
		}
		// Et chacun doit effectivement s'en servir, sinon le test ci-dessus
		// passerait pour un fichier qui ne construit plus rien du tout.
		if !strings.Contains(corps, "workloadBaseEnv") && !strings.Contains(corps, "artifactEndpointEnv") {
			t.Errorf("%s ne declare les points d'artefacts nulle part", nom)
		}
	}
}
