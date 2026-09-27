package notify

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/smtp"
	"strings"
	"time"
)

// Delivering an alert by mail.
//
// A webhook assumes an operator already runs something that receives one, and
// EMSE does not: conditions were recorded for weeks and nobody was told. What
// that installation does have is a mail path that works - the bridge inside
// the cluster, relaying over HTTPS to a public host, built because the site
// closes every outbound SMTP port. Onboarding and password resets already
// travel it.
//
// So mail is a second destination rather than a replacement. An installation
// may have a webhook, a mailbox, both, or neither, and "neither" has to stay
// silent rather than fail.

// MailDestination is where an alert is posted, resolved at send time so an
// administrator can change it without a redeployment.
type MailDestination struct {
	Host string
	Port string
	From string
	To   string
	// StartTLS is off inside a cluster talking to its own relay, and on when
	// the server is somebody else's.
	StartTLS bool
}

func (d MailDestination) usable() bool {
	return strings.TrimSpace(d.Host) != "" &&
		strings.TrimSpace(d.From) != "" &&
		strings.TrimSpace(d.To) != ""
}

// WithMail adds a mailbox beside the webhook.
func (n *Notifier) WithMail(resolve func() MailDestination) *Notifier {
	if n != nil {
		n.mail = resolve
	}
	return n
}

func (n *Notifier) mailDestination() MailDestination {
	if n == nil || n.mail == nil {
		return MailDestination{}
	}
	return n.mail()
}

// sendMail posts one alert. Failures are logged and swallowed for the same
// reason the webhook's are: a broken alerting channel must not take down the
// operation that raised the alert.
func (n *Notifier) sendMail(ctx context.Context, alert Alert, instance string) {
	destination := n.mailDestination()
	if !destination.usable() {
		return
	}
	port := strings.TrimSpace(destination.Port)
	if port == "" {
		port = "25"
	}
	address := net.JoinHostPort(strings.TrimSpace(destination.Host), port)
	subject := alertTitle(alert, instance)
	// Written as a message rather than a payload: the reader is a person on a
	// phone, and what they need first is whether to get up.
	body := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s\r\n",
		strings.TrimSpace(destination.From), strings.TrimSpace(destination.To), subject, alert.Text)

	deadline, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		done <- deliver(address, destination, subject, []byte(body))
	}()
	select {
	case err := <-done:
		if err != nil {
			log.Printf("alert %q could not be mailed: %v", alert.Event, err)
		}
	case <-deadline.Done():
		// The goroutine finishes on its own; what must not happen is the
		// caller waiting on a relay that has stopped answering.
		log.Printf("alert %q timed out on the way to %s", alert.Event, address)
	}
}

func deliver(address string, destination MailDestination, _ string, message []byte) error {
	client, err := smtp.Dial(address)
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()
	if destination.StartTLS {
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(nil); err != nil {
				return err
			}
		}
	}
	if err := client.Mail(strings.TrimSpace(destination.From)); err != nil {
		return err
	}
	if err := client.Rcpt(strings.TrimSpace(destination.To)); err != nil {
		return err
	}
	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write(message); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}
	return client.Quit()
}
