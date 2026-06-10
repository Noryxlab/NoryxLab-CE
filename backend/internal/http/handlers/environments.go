package handlers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
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
	Category         string                `json:"category"`
	WorkspaceIDEs    []string              `json:"workspaceIdes"`
	DestinationImage string                `json:"destinationImage"`
	LatestBuildID    string                `json:"latestBuildId"`
	LatestStatus     string                `json:"latestStatus"`
	LatestGitRepo    string                `json:"latestGitRepository"`
	LatestGitRef     string                `json:"latestGitRef"`
	LatestDockerfile string                `json:"latestDockerfilePath"`
	LatestImageSize  string                `json:"latestImageSizeGiB,omitempty"`
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
	sizeCache := map[string]string{}
	for _, b := range builds {
		if projectFilter != "" && b.ProjectID != projectFilter {
			continue
		}
		if !h.hasProjectMembership(userID, b.ProjectID) {
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
				Category:         deriveEnvironmentCategory(destination),
				WorkspaceIDEs:    deriveWorkspaceIDEs(destination, b.DockerfilePath, b.DockerfileContent),
				DestinationImage: destination,
				Revisions:        []environmentRevision{},
			}
			itemsByKey[key] = item
		} else {
			item.WorkspaceIDEs = mergeWorkspaceIDEs(item.WorkspaceIDEs, deriveWorkspaceIDEs(destination, b.DockerfilePath, b.DockerfileContent))
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

	if projectFilter == "" || h.hasProjectMembership(userID, projectFilter) {
		addSystemEnvironment(itemsByKey, projectFilter, h.workspaceJupyterImage, systemEnvironmentDefinitions["system-jupyter"])
		addSystemEnvironment(itemsByKey, projectFilter, h.workspaceVSCodeImage, systemEnvironmentDefinitions["system-vscode"])
		addSystemEnvironment(itemsByKey, projectFilter, h.workspaceRStudioImage, systemEnvironmentDefinitions["system-rstudio"])
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
		if item.DestinationImage != "" {
			if cached, ok := sizeCache[item.DestinationImage]; ok {
				item.LatestImageSize = cached
			} else {
				size := h.lookupImageSizeGiB(item.DestinationImage)
				sizeCache[item.DestinationImage] = size
				item.LatestImageSize = size
			}
		}
		items = append(items, *item)
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].UpdatedAt.After(items[j].UpdatedAt)
	})
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

type harborArtifact struct {
	Size int64 `json:"size"`
}

func (h Handlers) lookupImageSizeGiB(destinationImage string) string {
	base := strings.TrimSpace(h.harborURL)
	if base == "" {
		return ""
	}
	user := strings.TrimSpace(h.harborUsername)
	pass := strings.TrimSpace(h.harborPassword)

	_, project, repository, reference, ok := splitHarborImageRef(destinationImage)
	if !ok {
		return ""
	}
	baseURL, err := url.Parse(base)
	if err != nil || baseURL.Host == "" {
		return ""
	}

	artPath := fmt.Sprintf(
		"%s/api/v2.0/projects/%s/repositories/%s/artifacts/%s",
		strings.TrimRight(base, "/"),
		url.PathEscape(project),
		url.PathEscape(repository),
		url.PathEscape(reference),
	)
	req, err := http.NewRequest(http.MethodGet, artPath, nil)
	if err != nil {
		return ""
	}
	if user != "" && pass != "" {
		req.SetBasicAuth(user, pass)
	}

	client := &http.Client{Timeout: 4 * time.Second}
	if h.harborInsecureSkipVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ""
	}

	var artifact harborArtifact
	if err := json.NewDecoder(resp.Body).Decode(&artifact); err != nil {
		return ""
	}
	if artifact.Size <= 0 {
		return ""
	}
	const gib = 1024 * 1024 * 1024
	value := float64(artifact.Size) / float64(gib)
	return strconv.FormatFloat(value, 'f', 2, 64)
}

func (h Handlers) deleteImageFromHarbor(destinationImage string) error {
	base := strings.TrimSpace(h.harborURL)
	if base == "" {
		return fmt.Errorf("harbor url is not configured")
	}
	_, project, repository, reference, ok := splitHarborImageRef(destinationImage)
	if !ok {
		return fmt.Errorf("invalid image reference: %s", destinationImage)
	}

	apiPath := fmt.Sprintf(
		"%s/api/v2.0/projects/%s/repositories/%s/artifacts/%s",
		strings.TrimRight(base, "/"),
		url.PathEscape(project),
		url.PathEscape(repository),
		url.PathEscape(reference),
	)
	req, err := http.NewRequest(http.MethodDelete, apiPath, nil)
	if err != nil {
		return err
	}
	user := strings.TrimSpace(h.harborUsername)
	pass := strings.TrimSpace(h.harborPassword)
	if user != "" && pass != "" {
		req.SetBasicAuth(user, pass)
	}

	client := &http.Client{Timeout: 6 * time.Second}
	if h.harborInsecureSkipVerify {
		client.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("harbor api returned status=%d", resp.StatusCode)
	}
	return nil
}

func splitHarborImageRef(image string) (host, project, repository, reference string, ok bool) {
	raw := strings.TrimSpace(image)
	if raw == "" {
		return "", "", "", "", false
	}
	firstSlash := strings.Index(raw, "/")
	if firstSlash <= 0 {
		return "", "", "", "", false
	}
	host = strings.TrimSpace(raw[:firstSlash])
	pathRef := strings.TrimSpace(raw[firstSlash+1:])
	if host == "" || pathRef == "" {
		return "", "", "", "", false
	}

	lastColon := strings.LastIndex(pathRef, ":")
	lastAt := strings.LastIndex(pathRef, "@")
	switch {
	case lastAt > 0:
		reference = strings.TrimSpace(pathRef[lastAt+1:])
		pathRef = strings.TrimSpace(pathRef[:lastAt])
	case lastColon > strings.LastIndex(pathRef, "/"):
		reference = strings.TrimSpace(pathRef[lastColon+1:])
		pathRef = strings.TrimSpace(pathRef[:lastColon])
	default:
		return "", "", "", "", false
	}
	if reference == "" {
		return "", "", "", "", false
	}

	parts := strings.Split(pathRef, "/")
	if len(parts) < 2 {
		return "", "", "", "", false
	}
	project = strings.TrimSpace(parts[0])
	repository = strings.TrimSpace(strings.Join(parts[1:], "/"))
	if project == "" || repository == "" {
		return "", "", "", "", false
	}
	return host, project, repository, reference, true
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

func deriveEnvironmentCategory(destination string) string {
	d := strings.ToLower(strings.TrimSpace(destination))
	// CE-managed baseline images are published under noryx-environments/noryx-*
	if strings.Contains(d, "/noryx-environments/noryx-") {
		return "system"
	}
	return "custom"
}

type systemEnvironmentDefinition struct {
	BuildID        string
	GitRepository  string
	GitRef         string
	DockerfilePath string
	WorkspaceIDEs  []string
}

var systemEnvironmentDefinitions = map[string]systemEnvironmentDefinition{
	"system-jupyter": {
		BuildID:        "system-jupyter",
		GitRepository:  "https://github.com/Noryxlab/NoryxLab-CE.git",
		GitRef:         "main",
		DockerfilePath: "environments/noryx-jupyter/Dockerfile",
		WorkspaceIDEs:  []string{"jupyter"},
	},
	"system-vscode": {
		BuildID:        "system-vscode",
		GitRepository:  "https://github.com/Noryxlab/NoryxLab-CE.git",
		GitRef:         "main",
		DockerfilePath: "environments/noryx-vscode/Dockerfile",
		WorkspaceIDEs:  []string{"vscode"},
	},
	"system-rstudio": {
		BuildID:        "system-rstudio",
		GitRepository:  "https://github.com/Noryxlab/NoryxLab-CE.git",
		GitRef:         "main",
		DockerfilePath: "environments/noryx-rstudio/Dockerfile",
		WorkspaceIDEs:  []string{"rstudio"},
	},
}

func addSystemEnvironment(items map[string]*environmentItem, projectID, image string, definition systemEnvironmentDefinition) {
	image = strings.TrimSpace(image)
	if image == "" {
		return
	}
	key := projectID + "|" + image
	revision := environmentRevision{
		BuildID:          definition.BuildID,
		Status:           "succeeded",
		GitRepository:    definition.GitRepository,
		GitRef:           definition.GitRef,
		DockerfilePath:   definition.DockerfilePath,
		ContextPath:      ".",
		DestinationImage: image,
	}
	if item, ok := items[key]; ok {
		item.Category = "system"
		item.WorkspaceIDEs = mergeWorkspaceIDEs(item.WorkspaceIDEs, definition.WorkspaceIDEs)
		item.LatestBuildID = definition.BuildID
		item.LatestStatus = "succeeded"
		item.LatestGitRepo = definition.GitRepository
		item.LatestGitRef = definition.GitRef
		item.LatestDockerfile = definition.DockerfilePath
		item.Revisions = append([]environmentRevision{revision}, item.Revisions...)
		return
	}
	items[key] = &environmentItem{
		ID:               key,
		ProjectID:        projectID,
		Name:             deriveEnvironmentName(image),
		Category:         "system",
		WorkspaceIDEs:    definition.WorkspaceIDEs,
		DestinationImage: image,
		LatestBuildID:    definition.BuildID,
		LatestStatus:     "succeeded",
		LatestGitRepo:    definition.GitRepository,
		LatestGitRef:     definition.GitRef,
		LatestDockerfile: definition.DockerfilePath,
		Revisions:        []environmentRevision{revision},
	}
}

func getSystemEnvironmentDefinition(buildID string) (systemEnvironmentDefinition, bool) {
	definition, ok := systemEnvironmentDefinitions[strings.TrimSpace(buildID)]
	return definition, ok
}

func deriveWorkspaceIDEs(values ...string) []string {
	joined := strings.ToLower(strings.Join(values, "\n"))
	ides := []string{}
	if strings.Contains(joined, "noryx-python") || strings.Contains(joined, "jupyter") {
		ides = append(ides, "jupyter")
	}
	if strings.Contains(joined, "noryx-python") || strings.Contains(joined, "vscode") || strings.Contains(joined, "openvscode") {
		ides = append(ides, "vscode")
	}
	if strings.Contains(joined, "rstudio") || strings.Contains(joined, "rocker/") {
		ides = append(ides, "rstudio")
	}
	return ides
}

func mergeWorkspaceIDEs(current, extra []string) []string {
	seen := map[string]bool{}
	for _, ide := range append(current, extra...) {
		ide = strings.ToLower(strings.TrimSpace(ide))
		if allowedWorkspaceIDEs[ide] {
			seen[ide] = true
		}
	}
	result := make([]string, 0, len(seen))
	for _, ide := range []string{"jupyter", "vscode", "rstudio"} {
		if seen[ide] {
			result = append(result, ide)
		}
	}
	return result
}

func (h Handlers) workspaceEnvironmentAllowed(projectID, image, ide string) bool {
	image = strings.TrimSpace(image)
	ide = strings.ToLower(strings.TrimSpace(ide))
	if image == "" || !allowedWorkspaceIDEs[ide] {
		return false
	}
	if (ide == "jupyter" && image == strings.TrimSpace(h.workspaceJupyterImage)) ||
		(ide == "vscode" && image == strings.TrimSpace(h.workspaceVSCodeImage)) ||
		(ide == "rstudio" && image == strings.TrimSpace(h.workspaceRStudioImage)) {
		return true
	}
	builds, err := h.buildStore.List()
	if err != nil {
		return false
	}
	for _, b := range builds {
		if b.ProjectID != projectID || strings.TrimSpace(b.DestinationImage) != image || strings.ToLower(strings.TrimSpace(b.Status)) != "succeeded" {
			continue
		}
		for _, supportedIDE := range deriveWorkspaceIDEs(b.DestinationImage, b.DockerfilePath, b.DockerfileContent) {
			if supportedIDE == ide {
				return true
			}
		}
	}
	return false
}
