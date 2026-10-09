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
		// Carried here, beside the theme, because the interface needs it on
		// every dataset screen and this is the one cheap call it already
		// makes. Resolved per request for the same reason the theme is: a
		// change takes effect without a redeployment.
		"datasetDownload": h.currentDatasetDownload(),
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

// currentDatasetDownload says whether files may leave for a local machine.
func (h Handlers) currentDatasetDownload() string {
	if h.localDownloadBlocked() {
		return settings.DatasetDownloadBlocked
	}
	return settings.DatasetDownloadAllowed
}
