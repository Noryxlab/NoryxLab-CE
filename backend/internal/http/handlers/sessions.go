package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/session"
)

const sessionTTL = 8 * time.Hour

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
}

func (h Handlers) DeleteWebSession(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionCookie)
	if err == nil && strings.TrimSpace(cookie.Value) != "" && h.sessionStore != nil {
		_ = h.sessionStore.Delete(strings.TrimSpace(cookie.Value))
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
}
