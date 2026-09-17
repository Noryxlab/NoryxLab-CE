package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func newTestRelay() *relay {
	return &relay{
		token:          "jeton-de-test",
		host:           "ssl0.example.net:587",
		username:       "u",
		password:       "p",
		allowedSenders: []string{"noreply@noryxlab.ai", "@emse.noryxlab.ai"},
		limiter:        newRateLimiter(5, time.Hour),
	}
}

func post(t *testing.T, service *relay, token string, message envelope) *httptest.ResponseRecorder {
	t.Helper()
	payload, _ := json.Marshal(message)
	request := httptest.NewRequest(http.MethodPost, "/v1/send", bytes.NewReader(payload))
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	recorder := httptest.NewRecorder()
	service.routes().ServeHTTP(recorder, request)
	return recorder
}

func valid() envelope {
	return envelope{
		From: "noreply@noryxlab.ai",
		To:   []string{"chercheur@inria.fr"},
		Data: []byte("Subject: Bienvenue\r\n\r\nVotre acces est pret.\r\n"),
	}
}

func TestSansJetonRien(t *testing.T) {
	if got := post(t, newTestRelay(), "", valid()).Code; got != http.StatusUnauthorized {
		t.Errorf("attendu 401, recu %d", got)
	}
	if got := post(t, newTestRelay(), "mauvais", valid()).Code; got != http.StatusUnauthorized {
		t.Errorf("attendu 401 sur mauvais jeton, recu %d", got)
	}
}

// La regle qui separe un relai d'un relai ouvert.
//
// Un jeton finit toujours par fuiter - c'est arrive hier sur une autre clef de
// ce meme systeme. Ce qui doit rester vrai apres la fuite, c'est qu'on ne peut
// pas s'en servir pour emettre au nom de n'importe qui.
func TestUnExpediteurNonAutoriseEstRefuse(t *testing.T) {
	service := newTestRelay()
	message := valid()
	message.From = "quelquun@ailleurs.com"
	if got := post(t, service, "jeton-de-test", message).Code; got != http.StatusForbidden {
		t.Errorf("attendu 403 pour un expediteur etranger, recu %d", got)
	}
}

func TestUnDomaineEntierPeutEtreAutorise(t *testing.T) {
	service := newTestRelay()
	for _, from := range []string{
		"noreply@noryxlab.ai",         // adresse exacte
		"keycloak@emse.noryxlab.ai",   // domaine autorise
		"Noryx <noreply@noryxlab.ai>", // forme avec nom affiche
	} {
		if !service.senderAllowed(from) {
			t.Errorf("%q aurait du etre autorise", from)
		}
	}
	for _, from := range []string{
		"noreply@noryxlab.ai.evil.com", // suffixe trompeur
		"autre@example.com",
	} {
		if service.senderAllowed(from) {
			t.Errorf("%q n'aurait pas du etre autorise", from)
		}
	}
}

func TestSansListeDExpediteursToutEstRefuse(t *testing.T) {
	// Une configuration vide ne doit jamais valoir "tout le monde".
	service := newTestRelay()
	service.allowedSenders = nil
	if service.senderAllowed("noreply@noryxlab.ai") {
		t.Error("une liste vide a laisse passer un expediteur")
	}
}

func TestLeDebitEstBorne(t *testing.T) {
	// Ce qui borne le cout d'une fuite de jeton : au-dela du quota, un 429 que
	// le pont traduit en 4xx, donc que l'emetteur retentera plus tard.
	service := newTestRelay()
	service.limiter = newRateLimiter(2, time.Hour)
	for i := 0; i < 2; i++ {
		if got := post(t, service, "jeton-de-test", valid()).Code; got == http.StatusTooManyRequests {
			t.Fatalf("limite atteinte trop tot au message %d", i+1)
		}
	}
	if got := post(t, service, "jeton-de-test", valid()).Code; got != http.StatusTooManyRequests {
		t.Errorf("attendu 429 au-dela du quota, recu %d", got)
	}
}

func TestUneEnveloppeIncompleteEstRefusee(t *testing.T) {
	service := newTestRelay()
	for name, message := range map[string]envelope{
		"sans expediteur":   {To: []string{"a@b.fr"}, Data: []byte("x")},
		"sans destinataire": {From: "noreply@noryxlab.ai", Data: []byte("x")},
		"sans contenu":      {From: "noreply@noryxlab.ai", To: []string{"a@b.fr"}},
		"adresse illisible": {From: "noreply@noryxlab.ai", To: []string{"pas-une-adresse"}, Data: []byte("x")},
	} {
		t.Run(name, func(t *testing.T) {
			if got := post(t, service, "jeton-de-test", message).Code; got != http.StatusBadRequest {
				t.Errorf("attendu 400, recu %d", got)
			}
		})
	}
}
