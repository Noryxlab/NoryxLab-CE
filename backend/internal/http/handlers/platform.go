package handlers

import (
	"net/http"
	"time"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	storepkg "github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

func (h Handlers) GetPlatformOverview(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireIdentity(w, r); !ok {
		return
	}
	projects, projectErr := h.projectStore.List()
	datasets, datasetErr := h.datasetStore.ListAll()
	workspaces, workspaceErr := h.workspaceStore.List()
	apps, appErr := h.appStore.List()
	jobs, jobErr := h.jobStore.List()
	builds, buildErr := h.buildStore.List()
	if projectErr != nil || datasetErr != nil || workspaceErr != nil || appErr != nil || jobErr != nil || buildErr != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to build platform overview"})
		return
	}
	datasets = h.filterDatasetsForEdition(datasets)

	userCount := 0
	if h.keycloak != nil {
		if users, err := h.keycloak.ListUsers(); err == nil {
			userCount = len(users)
		}
	}
	active := 0
	for _, status := range appendExecutionStatuses(workspaces, apps, jobs, builds) {
		if status == "running" || status == "launching" || status == "submitted" {
			active++
		}
	}
	metrics := noryxruntime.WorkloadMetrics{}
	if inspector, ok := h.runtime.(noryxruntime.WorkloadMetricsInspector); ok {
		metrics, _ = inspector.GetWorkloadMetrics()
	}

	// What this figure covers, and what it leaves out.
	//
	// Read from what the nightly sweep measured, not walked here.
	//
	// It used to walk every bucket while somebody waited for the page, which
	// is why it carried a deadline, stopped halfway on large datasets and left
	// regulated ones out altogether. That exclusion was explained as refusing
	// to enumerate health data; it was really refusing to spend ten seconds -
	// the platform measures those same buckets on demand elsewhere, and one of
	// them is 24,000 objects and 400 GB. Summing object sizes reads no
	// content.
	//
	// So the sweep measures everything once a night and this reports it. The
	// figure is a total rather than a sample, and it carries the time it was
	// taken, because "as of last night" is honest in a way a number with no
	// date is not.
	var storageBytes int64
	storageDatasets := 0
	storageUnreadable := 0
	storagePending := 0
	var measuredAt time.Time
	sizes := map[string]storepkg.DatasetSize{}
	if h.datasetSizeStore != nil {
		if entries, err := h.datasetSizeStore.List(); err == nil {
			for _, entry := range entries {
				sizes[entry.DatasetID] = entry
			}
		}
	}
	for _, item := range datasets {
		entry, found := sizes[item.ID]
		switch {
		case !found:
			// Never measured yet - a platform that started an hour ago, or a
			// dataset created since the last sweep. Not the same as unreadable,
			// and saying so keeps the first night from looking like a fault.
			storagePending++
		case entry.Failure != "":
			storageUnreadable++
		default:
			storageBytes += entry.Bytes
			storageDatasets++
			if entry.MeasuredAt.After(measuredAt) {
				measuredAt = entry.MeasuredAt
			}
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sampledAt": time.Now().UTC(),
		"counts": map[string]int{
			"users":    userCount,
			"projects": len(projects),
			"datasets": len(datasets),
			"active":   active,
		},
		"workloadMetrics":   metrics,
		"storageMeasuredAt": measuredAtOrNil(measuredAt),
		"storage": map[string]any{
			"bytes":            storageBytes,
			"datasetsMeasured": storageDatasets,
			"datasetsTotal":    len(datasets),
			// Not yet measured, as distinct from measured and unreadable.
			"datasetsPending": storagePending,
			// Measured and unreadable are different facts with different
			// remedies, and a regulated dataset is no longer a third case:
			// the nightly sweep measures those too.
			"datasetsUnreadable": storageUnreadable,
		},
	})
}

// measuredAtOrNil keeps an unmeasured platform from reporting the zero time,
// which renders as the year one and reads as a bug rather than as "not yet".
func measuredAtOrNil(at time.Time) any {
	if at.IsZero() {
		return nil
	}
	return at
}
