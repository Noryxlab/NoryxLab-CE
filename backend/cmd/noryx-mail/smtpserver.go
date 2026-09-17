package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"time"
)

// Le minimum de SMTP qu'un emetteur attend.
//
// Keycloak envoie les invitations et les reinitialisations de mot de passe, et
// il ne sait parler que SMTP. Sur un site dont le filtre de sortie ferme les
// ports de messagerie - c'est le cas d'EMSE, verifie vers trois fournisseurs
// differents - il ne peut joindre aucun relai externe. Celui-ci vit dans le
// cluster, donc aucune sortie n'est requise, et transporte ensuite en HTTPS
// qui, lui, passe.
//
// Ce n'est pas un serveur de messagerie. Il n'a pas de file, pas de reessai,
// pas de remise locale : il accepte une enveloppe et la confie. Ce qu'il
// implemente est la conversation qu'un client mene reellement, et rien de
// plus - EHLO, MAIL FROM, RCPT TO, DATA, QUIT, plus RSET et NOOP que les
// clients envoient sans prevenir.
//
// Refuser tot est la regle : un serveur qui accepte DATA puis echoue laisse
// l'appelant croire que le message est parti.

const (
	// Assez pour une invitation avec un logo, pas assez pour servir de canal
	// d'exfiltration.
	maxMessageBytes = 5 << 20
	// Un client qui n'a rien dit depuis une minute a disparu.
	commandTimeout = 60 * time.Second
	maxRecipients  = 50
)

// deliverer est ce que le serveur fait d'une enveloppe complete. Le pont la
// pousse en HTTPS ; un test la garde en memoire.
type deliverer interface {
	deliver(envelope) error
}

type smtpServer struct {
	hostname string
	deliver  deliverer
}

func (s *smtpServer) serve(listener net.Listener) error {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return err
		}
		go func() {
			defer conn.Close()
			if err := s.handle(conn); err != nil && !errors.Is(err, io.EOF) {
				log.Printf("session smtp terminee: %v", err)
			}
		}()
	}
}

type session struct {
	from string
	to   []string
}

func (s *smtpServer) handle(conn net.Conn) error {
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)
	say := func(format string, args ...any) error {
		if _, err := fmt.Fprintf(writer, format+"\r\n", args...); err != nil {
			return err
		}
		return writer.Flush()
	}

	if err := say("220 %s noryx-mail", s.hostname); err != nil {
		return err
	}

	var current session
	for {
		_ = conn.SetReadDeadline(time.Now().Add(commandTimeout))
		line, err := reader.ReadString('\n')
		if err != nil {
			return err
		}
		command, argument := splitCommand(line)

		switch command {
		case "EHLO", "HELO":
			// Les capacites annoncees sont celles reellement tenues. Annoncer
			// STARTTLS sans le servir ferait echouer tout client qui le prend
			// au mot, et ce lien ne quitte pas le cluster.
			if err := say("250-%s\r\n250-SIZE %d\r\n250 8BITMIME", s.hostname, maxMessageBytes); err != nil {
				return err
			}
		case "MAIL":
			current = session{from: extractAddress(argument, "FROM:")}
			if err := say("250 2.1.0 ok"); err != nil {
				return err
			}
		case "RCPT":
			if len(current.to) >= maxRecipients {
				if err := say("452 4.5.3 trop de destinataires"); err != nil {
					return err
				}
				continue
			}
			current.to = append(current.to, extractAddress(argument, "TO:"))
			if err := say("250 2.1.5 ok"); err != nil {
				return err
			}
		case "DATA":
			if current.from == "" || len(current.to) == 0 {
				// Refuser avant le corps plutot qu'apres : accepter DATA puis
				// echouer laisse croire que le message est parti.
				if err := say("503 5.5.1 MAIL FROM et RCPT TO d'abord"); err != nil {
					return err
				}
				continue
			}
			if err := say("354 envoyez le message, terminez par un point seul"); err != nil {
				return err
			}
			body, err := readDotStuffed(reader)
			if err != nil {
				return err
			}
			message := envelope{From: current.from, To: current.to, Data: body}
			if err := message.validate(); err != nil {
				if err := say("550 5.1.0 %s", err); err != nil {
					return err
				}
				current = session{}
				continue
			}
			if err := s.deliver.deliver(message); err != nil {
				// 4xx et non 5xx : l'echec est le notre, pas celui de
				// l'appelant, et un client qui reessaie a raison de le faire.
				log.Printf("remise refusee: %v", err)
				if err := say("451 4.3.0 remise impossible pour le moment"); err != nil {
					return err
				}
				current = session{}
				continue
			}
			log.Printf("accepte de=%s vers=%d octets=%d", message.From, len(message.To), len(message.Data))
			if err := say("250 2.0.0 accepte"); err != nil {
				return err
			}
			current = session{}
		case "RSET":
			current = session{}
			if err := say("250 2.0.0 ok"); err != nil {
				return err
			}
		case "NOOP":
			if err := say("250 2.0.0 ok"); err != nil {
				return err
			}
		case "QUIT":
			_ = say("221 2.0.0 au revoir")
			return nil
		default:
			if err := say("502 5.5.2 commande inconnue"); err != nil {
				return err
			}
		}
	}
}

func splitCommand(line string) (command, argument string) {
	line = strings.TrimRight(line, "\r\n")
	parts := strings.SplitN(line, " ", 2)
	command = strings.ToUpper(strings.TrimSpace(parts[0]))
	if len(parts) > 1 {
		argument = strings.TrimSpace(parts[1])
	}
	return command, argument
}

// extractAddress lit l'adresse d'un MAIL FROM:<x> ou RCPT TO:<x>, en tolerant
// l'espace apres les deux-points et les parametres qui suivent.
func extractAddress(argument, prefix string) string {
	upper := strings.ToUpper(argument)
	if index := strings.Index(upper, prefix); index >= 0 {
		argument = argument[index+len(prefix):]
	}
	argument = strings.TrimSpace(argument)
	if start := strings.Index(argument, "<"); start >= 0 {
		if end := strings.Index(argument[start:], ">"); end > 0 {
			return argument[start+1 : start+end]
		}
	}
	if space := strings.IndexByte(argument, ' '); space > 0 {
		argument = argument[:space]
	}
	return argument
}

// readDotStuffed lit jusqu'au point seul et defait le point double.
//
// Le protocole prefixe d'un point toute ligne du corps qui en commence un, pour
// qu'elle ne soit pas prise pour la fin. Oublier de le defaire corrompt
// silencieusement tout message contenant une telle ligne - un extrait de code,
// une signature - et le defaut ne se voit que chez le destinataire.
func readDotStuffed(reader *bufio.Reader) ([]byte, error) {
	var body []byte
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return nil, err
		}
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "." {
			return body, nil
		}
		if strings.HasPrefix(trimmed, "..") {
			trimmed = trimmed[1:]
		}
		body = append(body, []byte(trimmed+"\r\n")...)
		if len(body) > maxMessageBytes {
			return nil, fmt.Errorf("message au-dela de %d octets", maxMessageBytes)
		}
	}
}
