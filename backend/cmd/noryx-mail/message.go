package main

import (
	"errors"
	"fmt"
	"net/mail"
	"strings"
)

// Ce qui traverse le pont.
//
// Une enveloppe, pas un message : l'expediteur, les destinataires et le corps
// tel que l'emetteur l'a ecrit, en-tetes compris. Le pont ne reecrit rien -
// Keycloak compose ses invitations avec ses propres en-tetes MIME, et un relai
// qui les reconstruirait casserait l'encodage des accents le jour ou personne
// ne regarde.
type envelope struct {
	// From est l'adresse d'enveloppe, celle qui decide du retour en cas
	// d'echec. Elle n'est pas forcement l'en-tete From: du message.
	From string `json:"from"`
	// To est la liste complete des destinataires, Cc et Cci compris : le
	// protocole les transporte separement du corps, et un Cci qui apparaitrait
	// dans les en-tetes ne serait plus un Cci.
	To []string `json:"to"`
	// Data est le message brut, en-tetes et corps, tel que recu.
	Data []byte `json:"data"`
}

var (
	errNoSender     = errors.New("enveloppe sans expediteur")
	errNoRecipient  = errors.New("enveloppe sans destinataire")
	errEmptyMessage = errors.New("enveloppe sans contenu")
)

// validate refuse ce qui ne peut pas etre remis.
//
// Verifie ici plutot qu'au moment de la soumission : une enveloppe invalide
// rejetee par le fournisseur arrive comme une erreur SMTP dans un journal que
// personne ne lit, plusieurs secondes apres que l'appelant a cru avoir envoye.
func (e envelope) validate() error {
	if strings.TrimSpace(e.From) == "" {
		return errNoSender
	}
	if _, err := mail.ParseAddress(e.From); err != nil {
		return fmt.Errorf("expediteur illisible %q: %w", e.From, err)
	}
	if len(e.To) == 0 {
		return errNoRecipient
	}
	for _, recipient := range e.To {
		if _, err := mail.ParseAddress(recipient); err != nil {
			return fmt.Errorf("destinataire illisible %q: %w", recipient, err)
		}
	}
	if len(e.Data) == 0 {
		return errEmptyMessage
	}
	return nil
}

// addressOnly retire le nom affiche : le protocole veut <adresse>, pas
// "Nom <adresse>". Les deux formes arrivent, selon ce que l'emetteur a compose.
func addressOnly(address string) string {
	parsed, err := mail.ParseAddress(address)
	if err != nil {
		return strings.Trim(strings.TrimSpace(address), "<>")
	}
	return parsed.Address
}
