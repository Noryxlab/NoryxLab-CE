package postgres

import (
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// Autant d'arguments que de places dans la requete.
//
// Un `$15` avec quatorze arguments compile, passe tous les tests qui ne
// touchent pas de base, et casse en production a la premiere ecriture. C'est
// arrive le 2026-10-26 en ajoutant une colonne a la table des jobs : le
// lancement d'un job repondait 500 sur les deux clusters, et le journal
// d'audit ne portait que « Internal Server Error ».
//
// Ce controle lit le fichier et compare, pour chaque appel a Exec, le plus
// grand `$n` de la requete au nombre d'arguments qui suivent. Il ne remplace
// pas une base de test ; il attrape la faute que personne ne voit en relisant.
func TestChaqueRequeteRecoitAutantDArgumentsQuElleEnAttend(t *testing.T) {
	source, err := os.ReadFile("./store.go")
	if err != nil {
		t.Fatal(err)
	}
	texte := string(source)
	placeholder := regexp.MustCompile(`\$(\d+)`)

	for _, appel := range appelsExec(texte) {
		requete, arguments, ok := decoupeAppel(appel)
		if !ok {
			continue
		}
		plusGrand := 0
		for _, trouve := range placeholder.FindAllStringSubmatch(requete, -1) {
			if n, err := strconv.Atoi(trouve[1]); err == nil && n > plusGrand {
				plusGrand = n
			}
		}
		if plusGrand == 0 {
			continue
		}
		if arguments != plusGrand {
			extrait := strings.TrimSpace(requete)
			if len(extrait) > 90 {
				extrait = extrait[:90] + "..."
			}
			t.Errorf("la requete attend %d arguments et en recoit %d : %s", plusGrand, arguments, extrait)
		}
	}
}

// appelsExec rend le texte de chaque appel a db.Exec, parentheses equilibrees.
func appelsExec(texte string) []string {
	out := []string{}
	for _, marqueur := range []string{"s.db.Exec(", "tx.Exec(", "transaction.Exec("} {
		depuis := 0
		for {
			i := strings.Index(texte[depuis:], marqueur)
			if i < 0 {
				break
			}
			debut := depuis + i + len(marqueur)
			profondeur := 1
			j := debut
			for j < len(texte) && profondeur > 0 {
				switch texte[j] {
				case '(':
					profondeur++
				case ')':
					profondeur--
				case '`':
					suivant := strings.IndexByte(texte[j+1:], '`')
					if suivant < 0 {
						j = len(texte)
						continue
					}
					j += suivant + 1
				}
				j++
			}
			out = append(out, texte[debut:j-1])
			depuis = j
		}
	}
	return out
}

// decoupeAppel separe la requete de ses arguments et compte ces derniers au
// premier niveau de parentheses.
func decoupeAppel(appel string) (string, int, bool) {
	appel = strings.TrimSpace(appel)
	if !strings.HasPrefix(appel, "`") {
		return "", 0, false
	}
	fin := strings.IndexByte(appel[1:], '`')
	if fin < 0 {
		return "", 0, false
	}
	requete := appel[1 : fin+1]
	reste := strings.TrimSpace(appel[fin+2:])
	reste = strings.TrimPrefix(reste, ",")
	if strings.TrimSpace(reste) == "" {
		return requete, 0, true
	}
	profondeur := 0
	arguments := 1
	for i := 0; i < len(reste); i++ {
		switch reste[i] {
		case '(', '[', '{':
			profondeur++
		case ')', ']', '}':
			profondeur--
		case ',':
			if profondeur == 0 {
				arguments++
			}
		}
	}
	// Une virgule finale avant la parenthese fermante compte en trop.
	if strings.HasSuffix(strings.TrimSpace(reste), ",") {
		arguments--
	}
	return requete, arguments, true
}
