package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// Le pont : SMTP en entree, HTTPS en sortie.
//
// Il vit dans le cluster. Keycloak et Noryx le designent comme leur serveur
// SMTP et ne savent rien du reste - aucun des deux n'a ete modifie, et
// Keycloak ne saurait pas l'etre : il ne parle que SMTP.
//
// Ce qu'il traverse est un filtre de sortie qui ferme les ports de messagerie
// vers toutes les destinations, verifie sur trois fournisseurs differents. Le
// 443 passe.
type bridge struct {
	relayURL string
	token    string
	client   *http.Client
}

func newBridge(relayURL, token string) *bridge {
	return &bridge{
		relayURL: strings.TrimRight(relayURL, "/") + "/v1/send",
		token:    token,
		// Le client attend, mais pas indefiniment : une session SMTP ouverte
		// pendant qu'on espere un relai muet tient une connexion de Keycloak.
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (b *bridge) deliver(message envelope) error {
	payload, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("enveloppe illisible: %w", err)
	}
	request, err := http.NewRequest(http.MethodPost, b.relayURL, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+b.token)

	response, err := b.client.Do(request)
	if err != nil {
		return fmt.Errorf("relai injoignable: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		// Le corps de la reponse n'est pas remonte tel quel : il vient d'un
		// autre systeme et l'appelant SMTP n'en ferait rien. Le code suffit a
		// distinguer "refuse" de "en panne", et le detail est dans le journal
		// du relai.
		return fmt.Errorf("relai a repondu %d", response.StatusCode)
	}
	return nil
}
