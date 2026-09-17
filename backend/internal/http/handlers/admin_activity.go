package handlers

import (
	"log"
	"net/http"
	"strings"
	"time"
)

// What the platform was actually used for, over a period.
//
// Called activity rather than usage because /admin/usage already exists and
// answers a different question - vCPU-hours consumed per project, which is a
// cost conversation. This one is about people and actions. Registering it
// under the same path did not fail to compile; Go's mux panics on a duplicate
// pattern at startup, so it would have taken the platform down on deploy
// instead. TestEveryRouteIsRegisteredOnce now catches that.
//
// The audit has recorded every action since the first day; nothing read it
// back except one event at a time. The question this answers - "who used this,
// doing what, when" - is the one asked at the end of a pilot, and answering it
// from memory is how a project reports impressions instead of measurements.
//
// The window is bounded rather than free: a report is a page somebody reads,
// and a request for ten years of a large table is a way to take the platform
// down from the administration screen.

const (
	activityDefaultWindow = 30 * 24 * time.Hour
	activityMaxWindow     = 365 * 24 * time.Hour
	// Enough to see what a platform is used for; past that a table stops being
	// read and starts being scrolled.
	activityTopActions = 25
)

type activityDayResponse struct {
	Day    string `json:"day"`
	People int    `json:"people"`
	Events int    `json:"events"`
}

type activityActorResponse struct {
	Actor    string     `json:"actor"`
	Events   int        `json:"events"`
	LastSeen *time.Time `json:"lastSeen,omitempty"`
}

type activityActionResponse struct {
	Action string `json:"action"`
	Count  int    `json:"count"`
}

func (h Handlers) GetActivityReport(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "users"); !ok {
		return
	}
	if h.auditStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "no audit store is configured"})
		return
	}

	// Both ends from one instant. Deriving "since" from time.Now() and letting
	// something else stamp "until" later produces a window a hair longer than
	// the one asked for, which is invisible until a caller checks the arithmetic.
	now := time.Now().UTC()
	window := activityDefaultWindow
	if raw := strings.TrimSpace(r.URL.Query().Get("window")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		switch {
		case err != nil || parsed <= 0:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "window must be a positive duration, for example 7d or 720h"})
			return
		case parsed > activityMaxWindow:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "window is longer than a year"})
			return
		default:
			window = parsed
		}
	}
	since := now.Add(-window)

	report, err := h.auditStore.Usage(since, now)
	if err != nil {
		log.Printf("activity report: %v", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "the audit store did not answer"})
		return
	}

	daily := make([]activityDayResponse, 0, len(report.Daily))
	for _, day := range report.Daily {
		daily = append(daily, activityDayResponse{
			Day:    day.Day.UTC().Format("2006-01-02"),
			People: day.People,
			Events: day.Events,
		})
	}
	people := make([]activityActorResponse, 0, len(report.People))
	for _, actor := range report.People {
		entry := activityActorResponse{Actor: actor.Actor, Events: actor.Events}
		if !actor.LastSeen.IsZero() {
			at := actor.LastSeen
			entry.LastSeen = &at
		}
		people = append(people, entry)
	}
	actions := make([]activityActionResponse, 0, activityTopActions)
	for i, action := range report.Actions {
		if i >= activityTopActions {
			break
		}
		actions = append(actions, activityActionResponse{Action: action.Action, Count: action.Count})
	}

	response := map[string]any{
		"since":        since,
		"until":        now,
		"totalEvents":  report.TotalEvents,
		"people":       people,
		"actions":      actions,
		"daily":        daily,
		"actionsShown": len(actions),
		"actionsTotal": len(report.Actions),
	}
	// What the audit holds, so the screen can tell a quiet period from a period
	// it has no records for. Absent when the store is empty, which is itself
	// the honest answer.
	if !report.CoversSince.IsZero() {
		response["coversSince"] = report.CoversSince
		response["coversUntil"] = report.CoversUntil
	}
	writeJSON(w, http.StatusOK, response)
}
