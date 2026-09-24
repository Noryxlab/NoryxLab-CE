package handlers

import (
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

// actorOrganizationSets maps an account to every organisation it belongs to.
//
// The directory allows a person in several organisations, and the singular
// map above keeps whichever it wrote last - which made the account list show
// one organisation per person, chosen by iteration order. A screen that draws
// people under their organisations has to see all of them or it misplaces
// somebody silently. The singular stays for the activity report, where one
// label per actor is the point and the choice is stated.
func (h Handlers) actorOrganizationSets() map[string][]string {
	membership := map[string][]string{}
	if h.keycloak == nil {
		return membership
	}
	organizations, err := h.keycloak.ListOrganizations()
	if err != nil {
		log.Printf("accounts: organisations unavailable, listing without them: %v", err)
		return membership
	}
	add := func(key, name string) {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			return
		}
		for _, existing := range membership[key] {
			if existing == name {
				return
			}
		}
		membership[key] = append(membership[key], name)
	}
	for _, organization := range organizations {
		members, err := h.keycloak.ListOrganizationMembers(organization.ID)
		if err != nil {
			continue
		}
		name := strings.TrimSpace(organization.Name)
		if name == "" {
			name = organization.Alias
		}
		for _, member := range members {
			add(member.Username, name)
			add(member.Email, name)
		}
	}
	return membership
}

// actorOrganizations maps an account to the organisation it belongs to.
//
// Keyed in lower case because the audit records whatever the identity
// provider put in the token, and a directory that answers "Malinet.C" for an
// audit that says "malinetc" produces a report where half the people have no
// organisation and nobody can see why.
func (h Handlers) actorOrganizations() map[string]string {
	membership := map[string]string{}
	if h.keycloak == nil {
		return membership
	}
	organizations, err := h.keycloak.ListOrganizations()
	if err != nil {
		log.Printf("activity report: organisations unavailable, reporting without them: %v", err)
		return membership
	}
	for _, organization := range organizations {
		members, err := h.keycloak.ListOrganizationMembers(organization.ID)
		if err != nil {
			continue
		}
		name := strings.TrimSpace(organization.Name)
		if name == "" {
			name = organization.Alias
		}
		for _, member := range members {
			if member.Username != "" {
				membership[strings.ToLower(member.Username)] = name
			}
			if member.Email != "" {
				membership[strings.ToLower(member.Email)] = name
			}
		}
	}
	return membership
}

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
	Actor string `json:"actor"`
	// Organization is empty for an account that belongs to none, and for a
	// machine account. Empty rather than "unknown": the platform is saying it
	// has no organisation for this actor, not that one exists and was not
	// found.
	Organization string     `json:"organization,omitempty"`
	Events       int        `json:"events"`
	LastSeen     *time.Time `json:"lastSeen,omitempty"`
}

// activityOrganizationResponse rolls the people up.
//
// The cut a pilot is reported in: an installation shared between Essilor, an
// engineering school and a research institute is asked how much each of them
// used it, not how much each account did. People is the honest headline -
// events are dominated by whoever automated something.
type activityOrganizationResponse struct {
	Organization string `json:"organization"`
	People       int    `json:"people"`
	Events       int    `json:"events"`
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
	// Who belongs where, resolved once for everybody rather than per actor:
	// one call per organisation instead of one per person, and a directory
	// that does not answer costs the column, not the report.
	membership := h.actorOrganizations()

	people := make([]activityActorResponse, 0, len(report.People))
	byOrganization := map[string]*activityOrganizationResponse{}
	for _, actor := range report.People {
		entry := activityActorResponse{
			Actor:        actor.Actor,
			Organization: membership[strings.ToLower(actor.Actor)],
			Events:       actor.Events,
		}
		if !actor.LastSeen.IsZero() {
			at := actor.LastSeen
			entry.LastSeen = &at
		}
		people = append(people, entry)

		roll := byOrganization[entry.Organization]
		if roll == nil {
			roll = &activityOrganizationResponse{Organization: entry.Organization}
			byOrganization[entry.Organization] = roll
		}
		roll.People++
		roll.Events += actor.Events
	}
	organizations := make([]activityOrganizationResponse, 0, len(byOrganization))
	for _, roll := range byOrganization {
		organizations = append(organizations, *roll)
	}
	// Most people first; the ones with no organisation sink to the bottom
	// whatever their count, because they are a residue rather than a party.
	sort.Slice(organizations, func(i, j int) bool {
		if (organizations[i].Organization == "") != (organizations[j].Organization == "") {
			return organizations[j].Organization == ""
		}
		if organizations[i].People != organizations[j].People {
			return organizations[i].People > organizations[j].People
		}
		return organizations[i].Organization < organizations[j].Organization
	})
	actions := make([]activityActionResponse, 0, activityTopActions)
	for i, action := range report.Actions {
		if i >= activityTopActions {
			break
		}
		actions = append(actions, activityActionResponse{Action: action.Action, Count: action.Count})
	}

	response := map[string]any{
		"since":         since,
		"until":         now,
		"totalEvents":   report.TotalEvents,
		"people":        people,
		"organizations": organizations,
		"actions":       actions,
		"daily":         daily,
		"actionsShown":  len(actions),
		"actionsTotal":  len(report.Actions),
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
