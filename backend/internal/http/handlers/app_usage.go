package handlers

import (
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

type appUsageVisitor struct {
	UserID     string    `json:"userId"`
	Views      int       `json:"views"`
	LastViewAt time.Time `json:"lastViewAt"`
}

type appUsageDay struct {
	Date  string `json:"date"`
	Views int    `json:"views"`
}

func (h Handlers) GetAppUsage(w http.ResponseWriter, r *http.Request) {
	record, _, ok := h.requireAppOperation(w, r, "app usage access")
	if !ok {
		return
	}
	if h.auditStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "app usage is not configured"})
		return
	}
	since := time.Now().UTC().AddDate(0, 0, -30)
	events, err := h.auditStore.List(store.AuditFilter{
		Since:      &since,
		Action:     "app.view",
		ResourceID: record.ID,
		Limit:      500,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read app usage"})
		return
	}

	visitorsByID := map[string]appUsageVisitor{}
	daysByDate := map[string]int{}
	anonymousViews := 0
	var lastViewedAt *time.Time
	for _, event := range events {
		day := event.OccurredAt.UTC().Format("2006-01-02")
		daysByDate[day]++
		if lastViewedAt == nil || event.OccurredAt.After(*lastViewedAt) {
			value := event.OccurredAt
			lastViewedAt = &value
		}
		userID := strings.TrimSpace(event.ActorUserID)
		if userID == "" || userID == "anonymous" {
			anonymousViews++
			continue
		}
		visitor := visitorsByID[userID]
		visitor.UserID = userID
		visitor.Views++
		if visitor.LastViewAt.IsZero() || event.OccurredAt.After(visitor.LastViewAt) {
			visitor.LastViewAt = event.OccurredAt
		}
		visitorsByID[userID] = visitor
	}

	visitors := make([]appUsageVisitor, 0, len(visitorsByID))
	for _, visitor := range visitorsByID {
		visitors = append(visitors, visitor)
	}
	sort.Slice(visitors, func(i, j int) bool { return visitors[i].LastViewAt.After(visitors[j].LastViewAt) })
	days := make([]appUsageDay, 0, len(daysByDate))
	for date, views := range daysByDate {
		days = append(days, appUsageDay{Date: date, Views: views})
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Date < days[j].Date })

	writeJSON(w, http.StatusOK, map[string]any{
		"appId":              record.ID,
		"periodDays":         30,
		"totalViews":         len(events),
		"identifiedVisitors": len(visitors),
		"anonymousViews":     anonymousViews,
		"lastViewedAt":       lastViewedAt,
		"visitors":           visitors,
		"daily":              days,
	})
}
