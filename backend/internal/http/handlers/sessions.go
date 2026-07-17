package handlers

import (
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/session"
)

const sessionTTL = 8 * time.Hour

var directLoginTemplate = template.Must(template.New("direct-login").Parse(`<!doctype html>
<html lang="fr">
  <head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <title>Connexion</title>
    <script src="/vendor/keycloak.js"></script>
    <style>
      body { margin:0; min-height:100vh; display:grid; place-items:center; font-family:Arial,sans-serif; background:#eef5f8; color:#142033; }
      main { max-width:520px; padding:32px; border:1px solid #d5e1ea; border-radius:18px; background:white; box-shadow:0 16px 40px rgba(15,23,42,.08); }
      h1 { margin:0 0 12px; font-size:28px; }
      p { margin:0; color:#52637a; line-height:1.5; }
    </style>
  </head>
  <body>
    <main>
      <h1>Connexion requise</h1>
      <p>Authentification en cours. Vous serez renvoye automatiquement vers l'application.</p>
    </main>
    <script>
      (async function () {
        const returnTo = {{ .ReturnTo }};
        try {
          const keycloak = new Keycloak({
            url: window.location.origin + '/auth',
            realm: 'noryx',
            clientId: 'noryx-api'
          });
          const authenticated = await keycloak.init({
            onLoad: 'login-required',
            checkLoginIframe: false,
            redirectUri: window.location.href
          });
          if (!authenticated) {
            await keycloak.login({ redirectUri: window.location.href });
            return;
          }
          await keycloak.updateToken(30).catch(function () {});
          const response = await fetch('/api/v1/auth/session', {
            method: 'POST',
            headers: { Authorization: 'Bearer ' + keycloak.token },
            credentials: 'same-origin'
          });
          if (!response.ok) {
            const text = await response.text().catch(function () { return ''; });
            throw new Error(text || 'session creation failed');
          }
          window.location.replace(returnTo || '/');
        } catch (error) {
          document.querySelector('p').textContent = 'Erreur de connexion: ' + String(error && error.message ? error.message : error);
        }
      })();
    </script>
  </body>
</html>`))

func (h Handlers) DirectLogin(w http.ResponseWriter, r *http.Request) {
	returnTo := safeLocalReturnTo(r.URL.Query().Get("returnTo"))
	if returnTo == "" {
		returnTo = "/"
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := directLoginTemplate.Execute(w, map[string]any{"ReturnTo": template.JSStr(returnTo)}); err != nil {
		http.Error(w, "failed to render login page", http.StatusInternalServerError)
		return
	}
}

func (h Handlers) CreateWebSession(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}

	if h.sessionStore == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "session store is not configured"})
		return
	}

	token := shortID() + shortID() + shortID()
	userID := identity.UserID()
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid authenticated identity"})
		return
	}

	expiresAt := time.Now().UTC().Add(sessionTTL)
	if err := h.sessionStore.Create(session.Session{
		Token:     token,
		Identity:  userID,
		ExpiresAt: expiresAt,
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create authenticated session"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  expiresAt,
	})

	writeJSON(w, http.StatusCreated, map[string]any{
		"expiresAt": expiresAt,
	})
	h.emitAudit(r, userID, "auth.login", "session", token, "", "success", "", map[string]any{
		"expiresAt": expiresAt,
	})
}

func (h Handlers) DeleteWebSession(w http.ResponseWriter, r *http.Request) {
	actorUserID, _ := h.userIDFromSessionOrBearerNoWrite(r)
	cookie, err := r.Cookie(sessionCookie)
	sessionToken := ""
	sessionIdentity := ""
	if err == nil && strings.TrimSpace(cookie.Value) != "" && h.sessionStore != nil {
		sessionToken = strings.TrimSpace(cookie.Value)
		if existing, ok, getErr := h.sessionStore.Get(sessionToken); getErr == nil && ok {
			sessionIdentity = strings.TrimSpace(existing.Identity)
		}
		_ = h.sessionStore.Delete(sessionToken)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})

	w.WriteHeader(http.StatusNoContent)
	if actorUserID == "" {
		actorUserID = sessionIdentity
	}
	h.emitAudit(r, actorUserID, "auth.logout", "session", sessionToken, "", "success", "", nil)
}

func safeLocalReturnTo(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" || !strings.HasPrefix(raw, "/") || strings.HasPrefix(raw, "//") {
		return ""
	}
	if strings.HasPrefix(raw, "/auth") || strings.HasPrefix(raw, "/api/v1/auth/login") {
		return ""
	}
	return raw
}
