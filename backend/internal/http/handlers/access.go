package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
)

const (
	userHeader      = "X-Noryx-User"
	authHeader      = "Authorization"
	globalAdminRole = "noryx-admin"
	sessionCookie   = "noryx_session"
)

func (h Handlers) requireIdentity(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	token := strings.TrimSpace(r.Header.Get(authHeader))
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimSpace(token)

	if token != "" && h.authVerifier != nil {
		identity, err := h.authVerifier.VerifyBearerToken(token)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid bearer token"})
			return auth.Identity{}, false
		}
		return identity, true
	}

	userID := strings.TrimSpace(r.Header.Get(userHeader))
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
		return auth.Identity{}, false
	}
	return auth.Identity{
		Username: userID,
		Roles:    map[string]struct{}{},
	}, true
}

func (h Handlers) requireUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return "", false
	}
	userID := identity.UserID()
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid authenticated identity"})
		return "", false
	}
	return userID, true
}

func (h Handlers) requireUserIDFromSessionOrBearer(w http.ResponseWriter, r *http.Request) (string, bool) {
	identity, ok := h.requireIdentityFromSessionOrBearer(w, r)
	if !ok {
		return "", false
	}
	userID := identity.UserID()
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid authenticated identity"})
		return "", false
	}
	return userID, true
}

func (h Handlers) userIDFromSessionOrBearerNoWrite(r *http.Request) (string, bool) {
	token := strings.TrimSpace(r.Header.Get(authHeader))
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimSpace(token)
	if token != "" {
		if h.authVerifier == nil {
			return "", false
		}
		identity, err := h.authVerifier.VerifyBearerToken(token)
		if err != nil {
			return "", false
		}
		userID := strings.TrimSpace(identity.UserID())
		if userID == "" {
			return "", false
		}
		return userID, true
	}

	if h.sessionStore == nil {
		return "", false
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return "", false
	}
	item, ok, err := h.sessionStore.Get(strings.TrimSpace(cookie.Value))
	if err != nil || !ok {
		return "", false
	}
	if time.Now().UTC().After(item.ExpiresAt) {
		_ = h.sessionStore.Delete(item.Token)
		return "", false
	}
	userID := strings.TrimSpace(item.Identity)
	if userID == "" {
		return "", false
	}
	return userID, true
}

func (h Handlers) requireIdentityFromSessionOrBearer(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	token := strings.TrimSpace(r.Header.Get(authHeader))
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimSpace(token)
	if token != "" {
		return h.requireIdentity(w, r)
	}

	if h.sessionStore == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
		return auth.Identity{}, false
	}

	cookie, err := r.Cookie(sessionCookie)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authenticated session"})
		return auth.Identity{}, false
	}

	item, ok, err := h.sessionStore.Get(strings.TrimSpace(cookie.Value))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read authenticated session"})
		return auth.Identity{}, false
	}
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid authenticated session"})
		return auth.Identity{}, false
	}
	if time.Now().UTC().After(item.ExpiresAt) {
		_ = h.sessionStore.Delete(item.Token)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "expired authenticated session"})
		return auth.Identity{}, false
	}

	return auth.Identity{
		Username: item.Identity,
		Roles:    map[string]struct{}{},
	}, true
}

func (h Handlers) requireProjectRole(
	w http.ResponseWriter,
	projectID string,
	userID string,
	check func(access.Role) bool,
	action string,
) bool {
	role, ok := h.accessStore.GetRole(projectID, userID)
	if !ok || !check(role) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient role for " + action})
		return false
	}
	return true
}

func (h Handlers) requireProjectMember(
	w http.ResponseWriter,
	projectID string,
	userID string,
	action string,
) bool {
	if _, ok := h.accessStore.GetRole(projectID, userID); !ok {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "project membership required for " + action})
		return false
	}
	return true
}

func (h Handlers) projectExists(projectID string) (bool, error) {
	projects, err := h.projectStore.List()
	if err != nil {
		return false, err
	}
	for _, p := range projects {
		if p.ID == projectID {
			return true, nil
		}
	}
	return false, nil
}

func (h Handlers) requireGlobalAdmin(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return auth.Identity{}, false
	}
	if h.isGlobalAdmin(identity) {
		return identity, true
	}
	writeJSON(w, http.StatusForbidden, map[string]string{"error": "global admin role required"})
	return auth.Identity{}, false
}

func (h Handlers) isGlobalAdmin(identity auth.Identity) bool {
	if identity.HasRole(globalAdminRole) {
		return true
	}
	if strings.TrimSpace(h.bootstrapAdminUser) != "" && identity.MatchesUsername(h.bootstrapAdminUser) {
		return true
	}
	if strings.TrimSpace(h.bootstrapAdminEmail) != "" && identity.MatchesEmail(h.bootstrapAdminEmail) {
		return true
	}
	return false
}

func bearerTokenFromHeader(r *http.Request) (string, error) {
	authz := strings.TrimSpace(r.Header.Get(authHeader))
	if authz == "" {
		return "", errors.New("missing Authorization header")
	}
	parts := strings.SplitN(authz, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("invalid Authorization format")
	}
	return strings.TrimSpace(parts[1]), nil
}
