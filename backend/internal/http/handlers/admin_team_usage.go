package handlers

import (
	"encoding/csv"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/usage"
)

// What each team consumed.
//
// Consumption is sampled per project and never per person, so a team's figure
// is an attribution rather than a measurement - and the honest way to publish
// one is to say which.
//
// The ambiguity is real: a project reached by two teams was paid for once and
// worked on by both. Splitting it in half would invent a precision nobody
// measured, and the halves would be wrong the moment one team did all the
// work. So each team is credited with the projects it reaches, in full, and
// the report states the overlap instead of hiding it. A reader comparing two
// teams gets a fair comparison; a reader adding them up is told the sum
// exceeds the platform, and by how much.
//
// It answers the question a finance team actually asks - which group is this
// cluster being bought for - without pretending the platform measured
// something it did not.

type teamUsageRow struct {
	TeamID   string `json:"teamId"`
	TeamName string `json:"teamName"`
	// Projects is how many the team reaches, and Members how many people are
	// in it: a large figure spread over thirty people reads differently from
	// the same figure spent by two.
	Projects int `json:"projects"`
	Members  int `json:"members"`

	VCPUHours      float64 `json:"vcpuHours"`
	MemoryGiBHours float64 `json:"memoryGibHours"`
	PeakVCPU       float64 `json:"peakVcpu"`
	// Samples carries through from the totals, so a figure built on three
	// measurements cannot be mistaken for one built on a month of them.
	Samples int `json:"samples"`
	// SharedProjects is how many of this team's projects another team also
	// reaches. It is the caveat attached to the row rather than to the page,
	// because it is true of some rows and not others.
	SharedProjects int `json:"sharedProjects"`
}

type teamUsageReport struct {
	From time.Time      `json:"from"`
	To   time.Time      `json:"to"`
	Rows []teamUsageRow `json:"rows"`
	// PlatformVCPUHours is what the platform actually consumed, so a reader
	// can see the double counting rather than deduce it.
	PlatformVCPUHours float64 `json:"platformVcpuHours"`
	// AttributedVCPUHours is the sum of the rows. Larger than the platform
	// total whenever teams share projects, which is the point of publishing
	// both.
	AttributedVCPUHours float64 `json:"attributedVcpuHours"`
	// UnattributedVCPUHours is what no team reaches - projects held
	// personally, or by an organization. Left visible because a report that
	// only shows what it can attribute quietly answers a different question.
	UnattributedVCPUHours float64 `json:"unattributedVcpuHours"`
}

func (h Handlers) GetAdminTeamUsage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireGlobalAdmin(w, r); !ok {
		return
	}
	from, to, valid := usageWindow(r)
	if !valid {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "from and to must be RFC3339 and no more than 400 days apart"})
		return
	}
	report, err := h.buildTeamUsageReport(from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read usage"})
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// GetAdminTeamUsageCSV is the same answer in the format a finance team opens.
func (h Handlers) GetAdminTeamUsageCSV(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireGlobalAdmin(w, r); !ok {
		return
	}
	from, to, valid := usageWindow(r)
	if !valid {
		writeJSON(w, http.StatusBadRequest,
			map[string]string{"error": "from and to must be RFC3339 and no more than 400 days apart"})
		return
	}
	report, err := h.buildTeamUsageReport(from, to)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read usage"})
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="noryx-team-usage.csv"`)
	writer := csv.NewWriter(w)
	defer writer.Flush()
	_ = writer.Write([]string{"team_id", "team_name", "members", "projects", "shared_projects",
		"vcpu_hours", "memory_gib_hours", "peak_vcpu", "samples", "from", "to"})
	for _, row := range report.Rows {
		_ = writer.Write([]string{
			row.TeamID, row.TeamName,
			strconv.Itoa(row.Members), strconv.Itoa(row.Projects), strconv.Itoa(row.SharedProjects),
			strconv.FormatFloat(row.VCPUHours, 'f', 3, 64),
			strconv.FormatFloat(row.MemoryGiBHours, 'f', 3, 64),
			strconv.FormatFloat(row.PeakVCPU, 'f', 3, 64),
			strconv.Itoa(row.Samples),
			report.From.Format(time.RFC3339), report.To.Format(time.RFC3339),
		})
	}
}

func (h Handlers) buildTeamUsageReport(from, to time.Time) (teamUsageReport, error) {
	report := teamUsageReport{From: from, To: to, Rows: []teamUsageRow{}}
	totals, err := h.usageTotals(from, to)
	if err != nil {
		return teamUsageReport{}, err
	}
	byProject := map[string]usage.Total{}
	for _, total := range totals {
		byProject[total.ProjectID] = total
		report.PlatformVCPUHours += total.VCPUHours
	}
	if h.teamStore == nil {
		// No teams configured: everything the platform consumed is
		// unattributed, which is the truthful answer rather than an empty one.
		report.UnattributedVCPUHours = report.PlatformVCPUHours
		return report, nil
	}

	// Which teams reach which project. Read per project because that is how
	// the grants are stored, and because the alternative - one query for all
	// grants - is a store method that exists only for this report.
	teamsByProject := map[string][]string{}
	reachedProjects := map[string]bool{}
	rows := map[string]*teamUsageRow{}
	for projectID := range byProject {
		grants, err := h.teamStore.ListProjectRoles(projectID)
		if err != nil {
			continue
		}
		for _, grant := range grants {
			teamsByProject[projectID] = append(teamsByProject[projectID], grant.TeamID)
			reachedProjects[projectID] = true
			row, seen := rows[grant.TeamID]
			if !seen {
				row = &teamUsageRow{
					TeamID:   grant.TeamID,
					TeamName: firstNonEmpty(grant.TeamName, grant.TeamID),
				}
				if members, err := h.teamStore.ListMembers(grant.TeamID); err == nil {
					row.Members = len(members)
				}
				rows[grant.TeamID] = row
			}
			total := byProject[projectID]
			row.Projects++
			row.VCPUHours += total.VCPUHours
			row.MemoryGiBHours += total.MemoryGiBHours
			row.Samples += total.Samples
			if total.PeakVCPU > row.PeakVCPU {
				row.PeakVCPU = total.PeakVCPU
			}
		}
	}

	// The overlap, counted once the whole picture is known: a project is
	// shared when more than one team reaches it.
	for projectID, teams := range teamsByProject {
		if len(teams) < 2 {
			continue
		}
		for _, teamID := range teams {
			if row, ok := rows[teamID]; ok {
				_ = projectID
				row.SharedProjects++
			}
		}
	}

	for _, row := range rows {
		report.AttributedVCPUHours += row.VCPUHours
		report.Rows = append(report.Rows, *row)
	}
	for projectID, total := range byProject {
		if !reachedProjects[projectID] {
			report.UnattributedVCPUHours += total.VCPUHours
		}
	}
	// Busiest first: a report read top to bottom should start with the row
	// somebody is going to ask about.
	sort.Slice(report.Rows, func(i, j int) bool {
		if report.Rows[i].VCPUHours != report.Rows[j].VCPUHours {
			return report.Rows[i].VCPUHours > report.Rows[j].VCPUHours
		}
		return strings.ToLower(report.Rows[i].TeamName) < strings.ToLower(report.Rows[j].TeamName)
	})
	return report, nil
}
