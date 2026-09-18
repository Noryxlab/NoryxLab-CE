package handlers

import (
	"encoding/json"
	"net/http"
)

func (h Handlers) GetHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	// Nothing this API returns may be reused without asking.
	//
	// The responses carried no cache directive at all, which leaves the
	// decision to a browser's heuristics and to whatever sits in front of the
	// platform. Every one of these answers is about state that changes -
	// which environments exist, what a project holds, who is a member - and a
	// stale one is not a slow screen, it is a screen showing a past that the
	// person then acts on. An environment list one revision behind offers an
	// image tag the platform no longer runs, and the launch is refused for a
	// choice nobody could see was wrong.
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
