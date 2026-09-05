package keycloak

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// SMTPSettings is the mail server a realm sends through.
//
// It lives in Keycloak rather than in the platform's own settings because
// Keycloak is what sends the mail: a copy here would be a second truth, and
// the one that matters would be the one nobody edited. The password is the
// exception in the other direction - it is written and never read back, so an
// administrator opening the screen cannot copy someone else's credential out
// of it.
type SMTPSettings struct {
	Host            string `json:"host"`
	Port            string `json:"port"`
	From            string `json:"from"`
	FromDisplayName string `json:"fromDisplayName"`
	ReplyTo         string `json:"replyTo"`
	User            string `json:"user"`
	// Auth says whether the server expects a user and a password at all. An
	// internal relay on a trusted network often does not.
	Auth     bool `json:"auth"`
	StartTLS bool `json:"starttls"`
	SSL      bool `json:"ssl"`
	// PasswordSet reports that a password is stored, without disclosing it.
	PasswordSet bool `json:"passwordSet"`
}

// Configured reports whether the realm can actually send mail. A host and a
// sender address are the minimum: Keycloak accepts a partial configuration and
// then fails at the moment somebody clicks "send", which is the worst place to
// find out.
func (s SMTPSettings) Configured() bool {
	return strings.TrimSpace(s.Host) != "" && strings.TrimSpace(s.From) != ""
}

func (c *Client) realmJSON(method string, payload any, output any) error {
	return c.adminJSON(method, "", payload, output)
}

// SMTP reads the realm's mail configuration, without its password.
func (c *Client) SMTP() (SMTPSettings, error) {
	var realm struct {
		SMTPServer map[string]string `json:"smtpServer"`
	}
	if err := c.realmJSON(http.MethodGet, nil, &realm); err != nil {
		return SMTPSettings{}, err
	}
	raw := realm.SMTPServer
	settings := SMTPSettings{
		Host:            raw["host"],
		Port:            raw["port"],
		From:            raw["from"],
		FromDisplayName: raw["fromDisplayName"],
		ReplyTo:         raw["replyTo"],
		User:            raw["user"],
		Auth:            raw["auth"] == "true",
		StartTLS:        raw["starttls"] == "true",
		SSL:             raw["ssl"] == "true",
		PasswordSet:     strings.TrimSpace(raw["password"]) != "",
	}
	return settings, nil
}

// SetSMTP writes the realm's mail configuration. An empty password keeps the
// stored one, so saving a change of port does not silently erase the
// credential.
func (c *Client) SetSMTP(settings SMTPSettings, password string) error {
	var realm struct {
		SMTPServer map[string]string `json:"smtpServer"`
	}
	if err := c.realmJSON(http.MethodGet, nil, &realm); err != nil {
		return err
	}
	existing := realm.SMTPServer
	if existing == nil {
		existing = map[string]string{}
	}
	updated := smtpMap(settings)
	if strings.TrimSpace(password) != "" {
		updated["password"] = password
	} else if kept, ok := existing["password"]; ok && kept != "" {
		updated["password"] = kept
	}
	return c.realmJSON(http.MethodPut, map[string]any{"smtpServer": updated}, nil)
}

// TestSMTP asks Keycloak to send a test message to the given address using the
// settings supplied - which may differ from what is stored, so an
// administrator can prove a change works before saving it.
func (c *Client) TestSMTP(settings SMTPSettings, password, recipient string) error {
	config := smtpMap(settings)
	if strings.TrimSpace(password) != "" {
		config["password"] = password
	} else {
		stored, err := c.SMTP()
		if err != nil {
			return err
		}
		if stored.PasswordSet {
			// Keycloak's own test endpoint accepts this placeholder and
			// substitutes the stored password, which is how its console avoids
			// sending the secret back to a browser.
			config["password"] = "**********"
		}
	}
	config["replyTo"] = settings.ReplyTo
	if recipient = strings.TrimSpace(recipient); recipient == "" {
		return fmt.Errorf("a recipient is required to test the mail server")
	}
	return c.adminJSON(http.MethodPost, "testSMTPConnection", config, nil)
}

func smtpMap(settings SMTPSettings) map[string]string {
	boolean := func(value bool) string {
		if value {
			return "true"
		}
		return "false"
	}
	return map[string]string{
		"host":            strings.TrimSpace(settings.Host),
		"port":            strings.TrimSpace(settings.Port),
		"from":            strings.TrimSpace(settings.From),
		"fromDisplayName": strings.TrimSpace(settings.FromDisplayName),
		"replyTo":         strings.TrimSpace(settings.ReplyTo),
		"user":            strings.TrimSpace(settings.User),
		"auth":            boolean(settings.Auth),
		"starttls":        boolean(settings.StartTLS),
		"ssl":             boolean(settings.SSL),
	}
}

// SendPasswordResetEmail asks Keycloak to mail the user a link that lets them
// set their own password.
//
// It is the half of account recovery that a temporary password cannot be: a
// password dictated over the phone travels through whoever is listening, and
// exists in a chat log afterwards. This one never leaves the mailbox it was
// sent to.
//
// Requires a mail server on the realm. Without one Keycloak fails the call,
// which is why the interface hides the action until SMTP is configured rather
// than offering a button that cannot work.
func (c *Client) SendPasswordResetEmail(userID string, lifespanSeconds int) error {
	path := "users/" + url.PathEscape(strings.TrimSpace(userID)) + "/execute-actions-email"
	if lifespanSeconds > 0 {
		path += fmt.Sprintf("?lifespan=%d", lifespanSeconds)
	}
	return c.adminJSON(http.MethodPut, path, []string{"UPDATE_PASSWORD"}, nil)
}
