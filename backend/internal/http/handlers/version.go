package handlers

import "net/http"

func (h Handlers) GetVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"version":        h.backendVersion,
		"backendVersion": h.backendVersion,
		"defaultTheme":   normalizeTheme(h.defaultTheme),
	})
}
