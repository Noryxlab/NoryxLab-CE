package handlers

import (
	"net/http"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/settings"
)

func (h Handlers) GetVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version":        h.backendVersion,
		"backendVersion": h.backendVersion,
		"edition":        h.edition,
		"defaultTheme":   normalizeTheme(h.currentDefaultTheme()),
	})
}

// currentDefaultTheme resolves the platform default on each request, so an
// operator changing it in the interface sees the effect without a restart -
// the same precedence every other setting has. Before this the stored value
// was never consulted at all.
func (h Handlers) currentDefaultTheme() string {
	if h.settings != nil {
		return h.settings.String(settings.KeyDefaultTheme)
	}
	return h.defaultTheme
}
