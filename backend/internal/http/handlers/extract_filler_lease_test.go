package handlers

import (
	"strings"
	"testing"
)

// L'absence de bail ne vaut pas abandon.
//
// Le kubelet cree trees/<nom> lui-meme, pour le subPath du conteneur principal,
// avant que le remplisseur du meme pod n'ait pu poser son bail. L'elagage
// prenait cette fenetre pour un residu : le remplisseur d'un workspace a efface
// l'arbre d'un autre demarre neuf secondes plus tot, dont le conteneur
// principal est mort en montant un chemin disparu sous lui.
func TestUnArbreSansBailNEstPasJugeAbandonne(t *testing.T) {
	script := extractFillerScript("/cache", "wks-moi")

	// Le bail reste la premiere protection.
	if !strings.Contains(script, `[ -n "$(find "$baux"/"$nom" -mmin -120 2>/dev/null)" ] && continue`) {
		t.Fatalf("un bail recent doit proteger l'arbre :\n%s", script)
	}
	// Et l'age du repertoire est la seconde, pour la fenetre de demarrage.
	if !strings.Contains(script, `[ -n "$(find "$arbre" -maxdepth 0 -mmin -120 2>/dev/null)" ] && continue`) {
		t.Fatalf("un repertoire jeune doit etre protege meme sans bail :\n%s", script)
	}
	// L'ancienne forme ne doit pas revenir : elle supprimait des que le bail
	// manquait, sans regarder l'arbre.
	if strings.Contains(script, `if [ -z "$(find "$baux"`) {
		t.Fatalf("l'ancien test « pas de bail donc abandonne » est revenu :\n%s", script)
	}
	// Son propre arbre n'est jamais candidat.
	if !strings.Contains(script, `[ "$arbre" = "$root" ] && continue`) {
		t.Fatalf("le remplisseur doit s'epargner lui-meme :\n%s", script)
	}
}

// Et le remplisseur tient son bail tant qu'il vit, sinon la protection
// ci-dessus expire au bout de deux heures sur un workspace encore ouvert.
func TestLeRemplisseurRenouvelleSonBail(t *testing.T) {
	script := extractFillerScript("/cache", "wks-moi")
	if !strings.Contains(script, `while true; do touch "$bail"`) {
		t.Fatalf("le bail doit etre renouvele en boucle :\n%s", script)
	}
}
