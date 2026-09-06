package handlers

import (
	"encoding/csv"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/usage"
)

// What a project consumed, and what everyone consumed.
//
// Two readings of the same samples: a project's own members see their history,
// and an administrator sees every project side by side - which is the view
// behind a cost conversation, and the one a customer asks for when they want
// to know why their bill moved.

const (
	defaultUsageWindow = 30 * 24 * time.Hour
	maxUsageWindow     = 400 * 24 * time.Hour
)

// usageWindow reads the from/to parameters, with a month as the default: the
// period a bill is written in.
func usageWindow(r *http.Request) (time.Time, time.Time, bool) {
	now := time.Now().UTC()
	to := now
	from := now.Add(-defaultUsageWindow)

	if raw := strings.TrimSpace(r.URL.Query().Get("from")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		from = parsed.UTC()
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("to")); raw != "" {
		parsed, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return time.Time{}, time.Time{}, false
		}
		to = parsed.UTC()
	}
	if !from.Before(to) {
		return time.Time{}, time.Time{}, false
	}
	// A window nobody has data for is a slow query and an empty answer; the
	// cap says so rather than letting it run.
	if to.Sub(from) > maxUsageWindow {
		return time.Time{}, time.Time{}, false
	}
	return from, to, true
}

func (h Handlers) GetProjectUsage(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if !h.requireProjectMember(w, projectID, identity.UserID(), "project usage") {
		return
	}
	from, to, valid := usageWindow(r)
	if !valid {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "from and to must be RFC3339 and no more than 400 days apart"})
		return
	}
	if h.usageStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"total": usage.Total{ProjectID: projectID, From: from, To: to}, "samples": []usage.Sample{}})
		return
	}
	samples, err := h.usageStore.ListByProject(projectID, from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read usage"})
		return
	}
	total := usage.Accumulate(projectID, samples, usageSampleInterval, usageMaxGap)
	total.From, total.To = from, to
	// The samples travel with the total so a chart needs one call, and so
	// anybody can check the arithmetic that produced the number.
	writeJSON(w, http.StatusOK, map[string]any{"total": total, "samples": samples})
}

func (h Handlers) GetPlatformUsage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireGlobalAdmin(w, r); !ok {
		return
	}
	from, to, valid := usageWindow(r)
	if !valid {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "from and to must be RFC3339 and no more than 400 days apart"})
		return
	}
	totals, err := h.usageTotals(from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read usage"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"from": from, "to": to, "items": totals})
}

// GetPlatformUsageCSV is the same answer in the format a finance team opens.
func (h Handlers) GetPlatformUsageCSV(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireGlobalAdmin(w, r); !ok {
		return
	}
	from, to, valid := usageWindow(r)
	if !valid {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "from and to must be RFC3339 and no more than 400 days apart"})
		return
	}
	totals, err := h.usageTotals(from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read usage"})
		return
	}
	names := map[string]string{}
	if projects, err := h.projectStore.List(); err == nil {
		for _, project := range projects {
			names[project.ID] = project.Name
		}
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="noryx-usage.csv"`)
	writer := csv.NewWriter(w)
	defer writer.Flush()
	_ = writer.Write([]string{"project", "project id", "from", "to", "vcpu hours", "memory gib hours", "peak vcpu", "peak memory gib", "samples"})
	for _, total := range totals {
		_ = writer.Write([]string{
			names[total.ProjectID], total.ProjectID,
			total.From.Format(time.RFC3339), total.To.Format(time.RFC3339),
			strconv.FormatFloat(total.VCPUHours, 'f', 2, 64),
			strconv.FormatFloat(total.MemoryGiBHours, 'f', 2, 64),
			strconv.FormatFloat(total.PeakVCPU, 'f', 2, 64),
			strconv.FormatFloat(total.PeakMemoryGiB, 'f', 2, 64),
			strconv.Itoa(total.Samples),
		})
	}
}

func (h Handlers) usageTotals(from, to time.Time) ([]usage.Total, error) {
	if h.usageStore == nil {
		return []usage.Total{}, nil
	}
	projects, err := h.usageStore.ListProjects(from, to)
	if err != nil {
		return nil, err
	}
	totals := make([]usage.Total, 0, len(projects))
	for _, projectID := range projects {
		samples, err := h.usageStore.ListByProject(projectID, from, to)
		if err != nil {
			return nil, err
		}
		total := usage.Accumulate(projectID, samples, usageSampleInterval, usageMaxGap)
		total.From, total.To = from, to
		totals = append(totals, total)
	}
	return totals, nil
}
