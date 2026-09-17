// noryx-mail traverse un site qui ferme le SMTP sortant.
//
// Le probleme, mesure plutot que suppose : sur l'installation EMSE, les ports
// 25, 465 et 587 sont fermes vers toutes les destinations testees - le relai de
// l'etablissement, Google, Microsoft et le fournisseur du domaine - pendant que
// le 443 sort normalement. C'est une politique anti-indesirables ordinaire et
// elle ne se negocie pas vite.
//
// Or les invitations et les reinitialisations de mot de passe ne viennent pas
// de la plateforme : elles viennent de Keycloak, qui ne parle que SMTP et ne
// peut pas etre modifie. Ajouter un transport HTTP a la plateforme aurait donc
// livre les alertes sans livrer l'onboarding, c'est-a-dire tout sauf ce qui
// etait demande.
//
// D'ou deux moities du meme binaire :
//
//	mode bridge  dans le cluster, ecoute en SMTP, pousse en HTTPS
//	mode relay   sur la machine publique, recoit en HTTPS, soumet au fournisseur
//
// Keycloak et la plateforme designent le pont comme leur serveur SMTP. Aucun
// des deux n'a ete modifie ; c'est de la configuration.
//
// Ce n'est pas un serveur de messagerie : pas de file, pas de reessai, pas de
// remise locale. Il accepte une enveloppe ou il la refuse, et un refus est un
// 4xx que l'emetteur sait retenter.
package main

import (
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func env(name, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(name)); value != "" {
		return value
	}
	return fallback
}

func main() {
	log.SetFlags(log.LstdFlags | log.LUTC)
	mode := strings.ToLower(env("NORYX_MAIL_MODE", ""))

	switch mode {
	case "bridge":
		if err := runBridge(); err != nil {
			log.Fatalf("pont: %v", err)
		}
	case "relay":
		if err := runRelay(); err != nil {
			log.Fatalf("relai: %v", err)
		}
	default:
		log.Fatalf("NORYX_MAIL_MODE doit valoir bridge ou relay (recu %q)", mode)
	}
}

func runBridge() error {
	relayURL := env("NORYX_MAIL_RELAY_URL", "")
	token := env("NORYX_MAIL_TOKEN", "")
	if relayURL == "" || token == "" {
		return errors.New("NORYX_MAIL_RELAY_URL et NORYX_MAIL_TOKEN sont requis")
	}
	address := env("NORYX_MAIL_LISTEN_ADDR", ":1025")
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	server := &smtpServer{
		hostname: env("NORYX_MAIL_HOSTNAME", "noryx-mail"),
		deliver:  newBridge(relayURL, token),
	}
	log.Printf("pont smtp sur %s, relai %s", address, relayURL)
	return server.serve(listener)
}

func runRelay() error {
	token := env("NORYX_MAIL_TOKEN", "")
	host := env("NORYX_MAIL_SMTP_HOST", "")
	username := env("NORYX_MAIL_SMTP_USERNAME", "")
	password := env("NORYX_MAIL_SMTP_PASSWORD", "")
	senders := strings.Split(env("NORYX_MAIL_ALLOWED_SENDERS", ""), ",")
	if token == "" || host == "" || username == "" || password == "" {
		return errors.New("NORYX_MAIL_TOKEN, _SMTP_HOST, _SMTP_USERNAME et _SMTP_PASSWORD sont requis")
	}
	// Sans liste d'expediteurs, ce service est un relai ouvert des que le jeton
	// fuit. Refuser de demarrer est la seule reponse qui ne se decouvre pas
	// trop tard.
	if strings.TrimSpace(strings.Join(senders, "")) == "" {
		return errors.New("NORYX_MAIL_ALLOWED_SENDERS est requis : sans lui ce relai est ouvert")
	}
	limit, err := strconv.Atoi(env("NORYX_MAIL_HOURLY_LIMIT", "200"))
	if err != nil || limit <= 0 {
		return fmt.Errorf("NORYX_MAIL_HOURLY_LIMIT doit etre un entier positif")
	}

	service := &relay{
		token: token, host: host, username: username, password: password,
		allowedSenders: senders,
		limiter:        newRateLimiter(limit, time.Hour),
	}
	address := env("NORYX_MAIL_LISTEN_ADDR", "127.0.0.1:8025")
	server := &http.Server{
		Addr:              address,
		Handler:           service.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("relai sur %s, soumission vers %s, %d messages/heure", address, host, limit)
	log.Printf("expediteurs autorises: %s", strings.Join(senders, " "))
	return server.ListenAndServe()
}
