package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
)

const userPrefLanguageKey = "language"
const userPrefThemeKey = "theme"

type updateUserPreferencesRequest struct {
	Language string `json:"language"`
	Theme    string `json:"theme"`
}

func normalizeLanguage(raw string) string {
	lang := strings.ToLower(strings.TrimSpace(raw))
	switch lang {
	case "fr", "fr-fr", "fr_ca", "fr-ca":
		return "fr"
	case "en", "en-us", "en_gb", "en-gb":
		return "en"
	default:
		return ""
	}
}

// normalizeTheme narrows the platform default to what the interface can act
// on: "light", "dark", or empty for "no platform default, follow the viewer".
//
// It used to accept only "noryx"/"default" and map everything else to empty,
// which rejected exactly the two values the setting offers and the interface
// understands. An operator could pick a default theme, see it saved, and watch
// nothing happen - three times over, since the setting also read a different
// environment variable from the one it declared and never consulted its own
// stored value.
//
// The legacy names are accepted and mean "no default": they were brand names,
// and branding moved to config.js when the frontend was rewritten (ADR-015).
func normalizeTheme(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "light":
		return "light"
	case "dark":
		return "dark"
	default:
		return ""
	}
}

func (h Handlers) GetUserPreferences(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	userID := identity.UserID()
	langValue, langFound, err := h.userPreferenceStore.Get(userID, userPrefLanguageKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read user preferences"})
		return
	}
	themeValue, themeFound, err := h.userPreferenceStore.Get(userID, userPrefThemeKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read user preferences"})
		return
	}
	out := map[string]any{}
	if langFound {
		if lang := normalizeLanguage(langValue); lang != "" {
			out["language"] = lang
		}
	}
	if themeFound {
		if theme := normalizeTheme(themeValue); theme != "" {
			out["theme"] = theme
		}
	}
	if h.keycloak != nil {
		identifier := strings.TrimSpace(identity.Subject)
		if identifier == "" {
			identifier = userID
		}
		if organizations, err := h.keycloak.ListUserOrganizations(identifier); err == nil {
			out["organizations"] = organizations
		}
	}
	if len(out) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func (h Handlers) UpdateUserPreferences(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	var req updateUserPreferencesRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	hasLanguage := strings.TrimSpace(req.Language) != ""
	hasTheme := strings.TrimSpace(req.Theme) != ""
	if !hasLanguage && !hasTheme {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one of language or theme is required"})
		return
	}
	out := map[string]any{}
	if hasLanguage {
		lang := normalizeLanguage(req.Language)
		if lang == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "language must be fr or en"})
			return
		}
		if err := h.userPreferenceStore.Set(userID, userPrefLanguageKey, lang); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update user preferences"})
			return
		}
		out["language"] = lang
	}
	if hasTheme {
		theme := normalizeTheme(req.Theme)
		if theme == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "theme must be noryx"})
			return
		}
		if err := h.userPreferenceStore.Set(userID, userPrefThemeKey, theme); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update user preferences"})
			return
		}
		out["theme"] = theme
	}
	writeJSON(w, http.StatusOK, out)
}
