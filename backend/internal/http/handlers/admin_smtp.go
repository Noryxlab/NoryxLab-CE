package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
)

// The mail server, as an administrator sets it.
//
// Until now a new account's temporary password was dictated over the phone or
// pasted into a chat, and Keycloak's own invitation and reset flows could not
// be used at all. Rather than pick a provider for every installation, the
// settings are a form: Essilor sends through their relay, EMSE through theirs,
// and a demo platform through whatever it has.

type smtpRequest struct {
	Host            string `json:"host"`
	Port            string `json:"port"`
	From            string `json:"from"`
	FromDisplayName string `json:"fromDisplayName"`
	ReplyTo         string `json:"replyTo"`
	User            string `json:"user"`
	Auth            bool   `json:"auth"`
	StartTLS        bool   `json:"starttls"`
	SSL             bool   `json:"ssl"`
	// Password is write-only. Empty means "keep the stored one", so changing a
	// port does not erase the credential.
	Password string `json:"password"`
	// TestRecipient is set by the test action only.
	TestRecipient string `json:"testRecipient"`
}

func (r smtpRequest) settings() keycloak.SMTPSettings {
	return keycloak.SMTPSettings{
		Host:            r.Host,
		Port:            r.Port,
		From:            r.From,
		FromDisplayName: r.FromDisplayName,
		ReplyTo:         r.ReplyTo,
		User:            r.User,
		Auth:            r.Auth,
		StartTLS:        r.StartTLS,
		SSL:             r.SSL,
	}
}

func (h Handlers) GetSMTPSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireGlobalAdmin(w, r); !ok {
		return
	}
	if h.keycloak == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no identity provider is configured"})
		return
	}
	settings, err := h.keycloak.SMTP()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the identity provider did not answer"})
		return
	}
	// The lifetime travels with the settings so the interface can name the
	// real number instead of carrying one of its own in a translated string.
	writeJSON(w, http.StatusOK, map[string]any{
		"settings":                  settings,
		"configured":                settings.Configured(),
		"passwordLinkLifetimeHours": int(h.passwordLinkLifetime.Hours()),
	})
}

func (h Handlers) UpdateSMTPSettings(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireGlobalAdmin(w, r)
	if !ok {
		return
	}
	if h.keycloak == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no identity provider is configured"})
		return
	}
	var req smtpRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid mail settings are required"})
		return
	}
	settings := req.settings()
	// Refused here rather than at the first click on "send a reset link": a
	// half-filled mail server is indistinguishable from a working one until
	// somebody is waiting for a message that never arrives.
	if strings.TrimSpace(settings.Host) != "" && !settings.Configured() {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a sender address is required alongside the host"})
		return
	}
	if settings.Auth && strings.TrimSpace(settings.User) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "authentication is enabled: a user is required"})
		return
	}
	if err := h.keycloak.SetSMTP(settings, req.Password); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the identity provider refused the mail settings"})
		return
	}
	stored, err := h.keycloak.SMTP()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the identity provider did not answer"})
		return
	}
	// The password is never in the audit entry, and never in the response.
	h.emitAudit(r, identity.UserID(), "smtp.update", "platform", "smtp", "", "success", "", map[string]any{
		"host": stored.Host, "port": stored.Port, "from": stored.From, "auth": stored.Auth, "starttls": stored.StartTLS, "ssl": stored.SSL,
	})
	writeJSON(w, http.StatusOK, map[string]any{"settings": stored, "configured": stored.Configured()})
}

func (h Handlers) TestSMTPSettings(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireGlobalAdmin(w, r)
	if !ok {
		return
	}
	if h.keycloak == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no identity provider is configured"})
		return
	}
	var req smtpRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid mail settings are required"})
		return
	}
	recipient := strings.TrimSpace(req.TestRecipient)
	if recipient == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a recipient is required to test the mail server"})
		return
	}
	if err := h.keycloak.TestSMTP(req.settings(), req.Password, recipient); err != nil {
		// The provider's own words are useful here - "connection refused",
		// "authentication failed" - and an administrator can act on them.
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the mail server refused the test: " + err.Error()})
		return
	}
	h.emitAudit(r, identity.UserID(), "smtp.test", "platform", "smtp", "", "success", "", map[string]any{"recipient": recipient})
	writeJSON(w, http.StatusOK, map[string]any{"sent": true, "recipient": recipient})
}

// SendUserPasswordResetEmail mails the user a link to set their own password.
//
// The alternative already exists - issue a temporary password and read it out -
// and it is the weaker one: the password travels through whoever is listening
// and stays in a chat log. This is offered only when the realm has a mail
// server, and the interface hides it otherwise rather than showing a button
// that fails.
func (h Handlers) SendUserPasswordResetEmail(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireAdminModule(w, r, "users")
	if !ok {
		return
	}
	if h.keycloak == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "keycloak admin client is not configured"})
		return
	}
	userID := strings.TrimSpace(r.PathValue("userID"))
	if userID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a user is required"})
		return
	}
	settings, err := h.keycloak.SMTP()
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the identity provider did not answer"})
		return
	}
	if !settings.Configured() {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "no mail server is configured: set one in the administration, or issue a temporary password"})
		return
	}
	// The window comes from configuration rather than from here. It used to be
	// twelve hours, reasoned as "long enough for somebody who reads their mail
	// the next morning" - true on a Tuesday, false for the case this is mostly
	// used for: an account created on a Friday afternoon and opened on Monday,
	// sixty hours later. An invitation is pushed to somebody who was not
	// waiting for it; a reset is pulled by somebody at their screen. One
	// duration cannot be right for both, so an installation sets it.
	lifespan := int(h.passwordLinkLifetime.Seconds())
	if lifespan <= 0 {
		lifespan = int((72 * time.Hour).Seconds())
	}
	// The client and address the person is returned to. Both already known to
	// the platform: the frontend client it issues tokens for, and the URL it
	// tells everybody else to use.
	if err := h.keycloak.SendPasswordResetEmail(userID, lifespan, h.oidcFrontendClientID, h.publicURL); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the mail could not be sent: " + err.Error()})
		return
	}
	h.emitAudit(r, identity.UserID(), "user.password.reset-email", "user", userID, "", "success", "", nil)
	writeJSON(w, http.StatusOK, map[string]any{"sent": true})
}
