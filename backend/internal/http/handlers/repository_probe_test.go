package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// La sonde d'un hote git ordinaire parle Basic, pas Bearer.
//
// Azure DevOps refuse un jeton d'acces personnel presente en bearer et repond
// par une redirection vers une page de connexion - ce qui, vu du formulaire,
// se lit comme un depot injoignable. Envoyer Bearer la etait donc un echec
// garanti pour la facon la plus naturelle de stocker un jeton : coller le
// jeton et rien d'autre.
//
// github.com et gitlab.com restent en bearer parce qu'ils sont sondes par
// leur API, ou un jeton nu *est* un bearer.
func TestLaSondeParleBasicSurUnHoteGitOrdinaire(t *testing.T) {
	for _, cas := range []struct {
		hote, secret string
		user, pass   string
		bearer       bool
	}{
		{"dev.azure.com", "jeton-nu", "oauth2", "jeton-nu", false},
		{"dev.azure.com", "monuser:jeton", "monuser", "jeton", false},
		{"dev.azure.com", ":jeton", "", "jeton", false},
		{"github.com", "jeton-nu", "", "", true},
		{"github.com", "monuser:jeton", "monuser", "jeton", false},
	} {
		req := httptest.NewRequest(http.MethodGet, "https://example.org/x", nil)
		applyRepositoryAuthHeaders(req, cas.hote, cas.secret)

		if cas.bearer {
			if got := req.Header.Get("Authorization"); got != "Bearer "+cas.secret {
				t.Errorf("%s/%q : Authorization = %q", cas.hote, cas.secret, got)
			}
			continue
		}
		user, pass, ok := req.BasicAuth()
		if !ok || user != cas.user || pass != cas.pass {
			t.Errorf("%s/%q : Basic %q:%q (ok=%v), attendu %q:%q",
				cas.hote, cas.secret, user, pass, ok, cas.user, cas.pass)
		}
	}
}

// Un secret vide n'authentifie rien, et ne doit pas inventer un identifiant.
func TestUnSecretVideNAuthentifiePas(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "https://example.org/x", nil)
	applyRepositoryAuthHeaders(req, "dev.azure.com", "   ")
	if req.Header.Get("Authorization") != "" {
		t.Errorf("Authorization = %q", req.Header.Get("Authorization"))
	}
}

// La meme lecture du secret que le clone, ecrite une fois.
func TestLeSecretSeLitDUneSeuleFacon(t *testing.T) {
	for secret, attendu := range map[string][2]string{
		"monuser:jeton": {"monuser", "jeton"},
		"jeton":         {"oauth2", "jeton"},
		":jeton":        {"", "jeton"},
		"  jeton  ":     {"oauth2", "jeton"},
	} {
		user, pass := splitRepositoryCredential(secret)
		if user != attendu[0] || pass != attendu[1] {
			t.Errorf("%q donne %q:%q, attendu %q:%q", secret, user, pass, attendu[0], attendu[1])
		}
	}
}

// Un 404 sur un depot prive ne prouve pas son absence.
//
// Le serveur cache l'existence plutot que de l'admettre : « absent » et « pas
// autorise » sont la meme reponse. Dire « repository not found » a quelqu'un
// qui a le depot ouvert dans un autre onglet est ce qui transforme une sonde
// correcte en sonde inutile - c'est ce que Samy a lu le 9 octobre.
func TestUn404NAffirmePasLAbsence(t *testing.T) {
	serveur := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "", http.StatusNotFound)
	}))
	t.Cleanup(serveur.Close)

	_, err := checkRepository(serveur.URL+"/org/projet/_git/depot", "jeton")
	if err == nil {
		t.Fatal("un 404 doit rester une erreur")
	}
	message := err.Error()
	if strings.Contains(message, "repository not found") {
		t.Errorf("le message ne doit plus affirmer l'absence : %q", message)
	}
	for _, attendu := range []string{"404", "does not exist", "cannot see it"} {
		if !strings.Contains(message, attendu) {
			t.Errorf("le message doit porter %q : %q", attendu, message)
		}
	}
}

// Un statut inattendu nomme l'adresse reellement interrogee.
//
// La sonde n'interroge pas l'URL du depot : elle en derive une. Comparer un
// statut surprenant a l'adresse qu'on a tapee, c'est comparer deux choses
// differentes.
func TestUnStatutInattenduNommeLAdresseSondee(t *testing.T) {
	serveur := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "", http.StatusTeapot)
	}))
	t.Cleanup(serveur.Close)

	_, err := checkRepository(serveur.URL+"/org/projet/_git/depot", "jeton")
	if err == nil || !strings.Contains(err.Error(), "info/refs") {
		t.Errorf("l'erreur doit nommer l'adresse sondee : %v", err)
	}
}
