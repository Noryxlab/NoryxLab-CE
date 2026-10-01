package postgres

import (
	"strings"
	"testing"
)

// La reprise ne cree pas de lien vers un projet disparu.
//
// Chaque extrait nommait un projet ; le rattachement est devenu un lien, et la
// migration recopie donc ce projet comme premiere ligne de lien. Elle le
// faisait sans regarder si le projet existait encore : deux extraits dont le
// projet avait ete supprime sur EMSE ont produit deux lignes que rien
// n'affiche et que le controle d'orphelins a signalees le jour meme.
//
// Un extrait dont le projet a disparu n'est rattache a rien, ce qui est la
// verite, et il reste rattachable depuis l'ecran du projet.
func TestLaRepriseIgnoreLesProjetsDisparus(t *testing.T) {
	reprise := ""
	for _, requete := range migrationStatements() {
		if strings.Contains(requete, "INSERT INTO project_extract_links") {
			reprise = requete
			break
		}
	}
	if reprise == "" {
		t.Fatal("la reprise des liens d'extrait a disparu des migrations")
	}
	if !strings.Contains(reprise, "FROM projects") {
		t.Fatalf("la reprise doit verifier que le projet existe encore :\n%s", reprise)
	}
}
