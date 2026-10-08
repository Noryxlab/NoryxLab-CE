package handlers

import (
	"strings"
	"testing"
)

func TestDatasetDirectObjectKeyStaysUnderDatasetPrefix(t *testing.T) {
	for _, test := range []struct {
		name, prefix, raw, want string
		ok                      bool
	}{
		{"nested object", "incoming", "study/0001/image.dcm", "incoming/study/0001/image.dcm", true},
		{"cleaned object", "incoming/", "study/../manifest.json", "incoming/manifest.json", true},
		{"empty is refused", "incoming", "", "", false},
		{"absolute is refused", "incoming", "/other/object", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := datasetDirectObjectKey(test.prefix, test.raw)
			if got != test.want || ok != test.ok {
				t.Fatalf("datasetDirectObjectKey(%q, %q) = (%q, %v), want (%q, %v)", test.prefix, test.raw, got, ok, test.want, test.ok)
			}
		})
	}
}

// Un chemin qui tente de sortir du dataset y reste.
//
// Le presign signe une cle que le serveur compose : c'est le seul endroit ou
// le confinement se decide, puisque l'objet part ensuite directement vers S3
// sans repasser par la plateforme.
func TestUnCheminNePeutPasSortirDuDataset(t *testing.T) {
	for _, essai := range []string{
		"../ailleurs/objet", "../../etc/passwd", "etude/../../ailleurs/objet", "./../x",
	} {
		cle, ok := datasetDirectObjectKey("incoming", essai)
		if !ok {
			continue
		}
		if !strings.HasPrefix(cle, "incoming/") {
			t.Errorf("%q est sorti du prefixe : %q", essai, cle)
		}
	}
}

// Le plafond d'objet s'applique aussi a la voie directe.
//
// La voie proxyfiee refuse un objet au-dela de maxDatasetObjectBytes et tient
// le flux a la longueur annoncee. Un PUT presigne ne passe par rien de tout
// ca : la plateforme signe une cle et ne voit jamais les octets, donc le seul
// plafond restant etait celui de S3 - le meme chiffre, par coincidence et non
// par decision, ecrit nulle part. La taille annoncee rend le controle
// applicable la ou il peut encore l'etre.
func TestLePlafondDObjetSAppliqueAuPresign(t *testing.T) {
	if probleme := datasetUploadSizeProblem(datasetUploadURLRequest{Path: "a.dcm", Size: 2 << 20}); probleme != "" {
		t.Fatalf("un objet ordinaire doit passer : %s", probleme)
	}
	if probleme := datasetUploadSizeProblem(datasetUploadURLRequest{Path: "a.dcm"}); probleme != "" {
		t.Fatalf("une taille non annoncee n'est pas un refus : %s", probleme)
	}
	probleme := datasetUploadSizeProblem(datasetUploadURLRequest{Path: "enorme.tar", Size: maxDatasetObjectBytes + 1})
	if probleme == "" {
		t.Fatal("un objet au-dela du plafond doit etre refuse")
	}
	// Le refus nomme le fichier : dans un lot de quatre mille, « trop gros »
	// sans nom ne sert a rien.
	if !strings.Contains(probleme, "enorme.tar") {
		t.Errorf("le refus doit nommer l'objet : %s", probleme)
	}
	if probleme := datasetUploadSizeProblem(datasetUploadURLRequest{Path: "a.dcm", Size: -1}); probleme == "" {
		t.Error("une taille negative doit etre refusee")
	}
}
