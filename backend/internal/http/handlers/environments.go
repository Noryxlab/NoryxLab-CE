package handlers

import (
	"net/http"
	"sort"
	"strings"
	"time"
)

type environmentRevision struct {
	BuildID          string    `json:"buildId"`
	JobName          string    `json:"jobName"`
	Status           string    `json:"status"`
	GitRepository    string    `json:"gitRepository"`
	GitRef           string    `json:"gitRef"`
	DockerfilePath   string    `json:"dockerfilePath"`
	ContextPath      string    `json:"contextPath"`
	DestinationImage string    `json:"destinationImage"`
	CreatedAt        time.Time `json:"createdAt"`
}

type environmentItem struct {
	ID               string                `json:"id"`
	ProjectID        string                `json:"projectId"`
	Name             string                `json:"name"`
	DestinationImage string                `json:"destinationImage"`
	LatestBuildID    string                `json:"latestBuildId"`
	LatestStatus     string                `json:"latestStatus"`
	LatestGitRepo    string                `json:"latestGitRepository"`
	LatestGitRef     string                `json:"latestGitRef"`
	LatestDockerfile string                `json:"latestDockerfilePath"`
	UpdatedAt        time.Time             `json:"updatedAt"`
	Revisions        []environmentRevision `json:"revisions"`
}

func (h Handlers) ListEnvironments(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	h.syncBuildsFromRuntime()

	projectFilter := strings.TrimSpace(r.URL.Query().Get("projectId"))
	builds, err := h.buildStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list builds"})
		return
	}

	itemsByKey := map[string]*environmentItem{}
	for _, b := range builds {
		if projectFilter != "" && b.ProjectID != projectFilter {
			continue
		}
		if _, allowed := h.accessStore.GetRole(b.ProjectID, userID); !allowed {
			continue
		}
		destination := strings.TrimSpace(b.DestinationImage)
		if destination == "" {
			continue
		}
		key := b.ProjectID + "|" + destination
		item, exists := itemsByKey[key]
		if !exists {
			item = &environmentItem{
				ID:               key,
				ProjectID:        b.ProjectID,
				Name:             deriveEnvironmentName(destination),
				DestinationImage: destination,
				Revisions:        []environmentRevision{},
			}
			itemsByKey[key] = item
		}

		rev := environmentRevision{
			BuildID:          b.ID,
			JobName:          b.JobName,
			Status:           b.Status,
			GitRepository:    b.GitRepository,
			GitRef:           b.GitRef,
			DockerfilePath:   b.DockerfilePath,
			ContextPath:      b.ContextPath,
			DestinationImage: b.DestinationImage,
			CreatedAt:        b.CreatedAt,
		}
		item.Revisions = append(item.Revisions, rev)
	}

	items := make([]environmentItem, 0, len(itemsByKey))
	for _, item := range itemsByKey {
		sort.SliceStable(item.Revisions, func(i, j int) bool {
			return item.Revisions[i].CreatedAt.After(item.Revisions[j].CreatedAt)
		})
		if len(item.Revisions) > 0 {
			latest := item.Revisions[0]
			item.LatestBuildID = latest.BuildID
			item.LatestStatus = latest.Status
			item.LatestGitRepo = latest.GitRepository
			item.LatestGitRef = latest.GitRef
			item.LatestDockerfile = latest.DockerfilePath
			item.UpdatedAt = latest.CreatedAt
		}
		items = append(items, *item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func deriveEnvironmentName(destination string) string {
	destination = strings.TrimSpace(destination)
	if destination == "" {
		return "environment"
	}
	parts := strings.Split(destination, "/")
	last := strings.TrimSpace(parts[len(parts)-1])
	if last == "" {
		return destination
	}
	return last
}
