package handlers

import "net/http"

const backendVersion = "0.5.67"

func (h Handlers) GetVersion(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"version": backendVersion})
}
