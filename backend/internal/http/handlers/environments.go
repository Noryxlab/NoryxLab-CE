package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/workspacekind"
)

type environmentRevision struct {
	// Number is what a person refers to: revision 3 of "training", not
	// a449755a. Counted per environment, oldest first, so it never changes
	// once assigned and a workload pinned to revision 2 keeps meaning the
	// same thing.
	Number           int       `json:"number"`
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
	ID               string   `json:"id"`
	ProjectID        string   `json:"projectId"`
	Name             string   `json:"name"`
	Category         string   `json:"category"`
	WorkspaceIDEs    []string `json:"workspaceIdes"`
	DestinationImage string   `json:"destinationImage"`
	LatestBuildID    string   `json:"latestBuildId"`
	LatestStatus     string   `json:"latestStatus"`
	LatestGitRepo    string   `json:"latestGitRepository"`
	LatestGitRef     string   `json:"latestGitRef"`
	LatestDockerfile string   `json:"latestDockerfilePath"`
	LatestImageSize  string   `json:"latestImageSizeGiB,omitempty"`
	// Vulnerabilities is what the registry found in the image this environment
	// runs. Absent when the registry does not scan: an empty report must never
	// read as a clean one.
	Vulnerabilities *imageVulnerabilities `json:"vulnerabilities,omitempty"`
	UpdatedAt       time.Time             `json:"updatedAt"`
	Revisions       []environmentRevision `json:"revisions"`
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
	scanCache := map[string]*imageVulnerabilities{}
	// The repositories the platform ships itself.
	//
	// A system environment is platform-wide and carries no project, while a
	// build carries the project that produced it. Keyed the same way, those two
	// never met: rebuilding a system image from a project produced a second row
	// with the same name, and the list grew one entry per rebuild instead of
	// one revision. A rebuild of a system image is a revision of that
	// environment, so it is keyed as one.
	systemRepositories := h.systemEnvironmentRepositories()
	keyFor := func(projectID, image string) string {
		repository := imageRepository(image)
		if systemRepositories[repository] {
			return "|" + repository
		}
		return projectID + "|" + repository
	}
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
		// Keyed by the repository and not the full reference. Every rebuild
		// pushes a new tag, so keying on the reference made each rebuild a
		// separate environment - the list filled up with what is one thing
		// built twice.
		key := keyFor(b.ProjectID, destination)
		item, exists := itemsByKey[key]
		if !exists {
			item = &environmentItem{
				ID:               key,
				ProjectID:        b.ProjectID,
				Name:             environmentDisplayName(b.Name, destination),
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
		// A registered kind offers its own image beside them. The screen that
		// lists environments is where a workspace is actually started, so a
		// kind absent from it exists only for whoever calls the API by hand.
		for _, kind := range workspacekind.All() {
			if kind.Catalogue == nil || kind.DefaultImage == nil {
				continue
			}
			image := strings.TrimSpace(kind.DefaultImage())
			entry := kind.Catalogue()
			if image == "" || strings.TrimSpace(entry.ID) == "" {
				continue
			}
			addSystemEnvironment(itemsByKey, projectFilter, image, systemEnvironmentDefinition{
				BuildID:        entry.ID,
				GitRepository:  entry.GitRepository,
				GitRef:         entry.GitRef,
				DockerfilePath: entry.DockerfilePath,
				WorkspaceIDEs:  []string{kind.ID},
			})
		}
	}

	items := make([]environmentItem, 0, len(itemsByKey))
	for _, item := range itemsByKey {
		numberEnvironmentRevisions(item)
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
			if cached, ok := scanCache[item.DestinationImage]; ok {
				item.Vulnerabilities = cached
			} else {
				report := h.resolveImageVulnerabilities(item.DestinationImage)
				scanCache[item.DestinationImage] = report
				item.Vulnerabilities = report
			}
		}
		items = append(items, *item)
	}

	// Most recently updated first, except that the platform's own environments
	// lead, VS Code at the top. The launch sheet preselects whatever comes
	// first, so this order is a default somebody lives with on every workspace
	// they open - and it was decided by which system image happened to be
	// rebuilt last.
	systemRank := map[string]int{"system-vscode": 0, "system-jupyter": 1, "system-rstudio": 2}
	rank := func(item environmentItem) int {
		if position, ok := systemRank[item.LatestBuildID]; ok {
			return position
		}
		return len(systemRank)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if rank(items[i]) != rank(items[j]) {
			return rank(items[i]) < rank(items[j])
		}
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

	client := h.harborHTTPClient(4 * time.Second)
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

	client := h.harborHTTPClient(6 * time.Second)
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

// imageRepository is an image reference without its tag or digest: what stays
// the same across rebuilds of one environment.
func imageRepository(reference string) string {
	reference = strings.TrimSpace(reference)
	if at := strings.Index(reference, "@"); at > 0 {
		reference = reference[:at]
	}
	slash := strings.LastIndex(reference, "/")
	if colon := strings.LastIndex(reference, ":"); colon > slash {
		reference = reference[:colon]
	}
	return reference
}

// environmentDisplayName prefers the name somebody typed. The image reference
// is the fallback for everything built before the platform remembered it, and
// for a build submitted through the API with no name at all.
func environmentDisplayName(name, destination string) string {
	if trimmed := strings.TrimSpace(name); trimmed != "" {
		return trimmed
	}
	return deriveEnvironmentName(destination)
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

// numberEnvironmentRevisions counts forward from the first build, then leaves
// the list newest first for the screen. The number has to mean "the third time
// this was built", which only counting forwards gives - and it must not move
// when a fourth build arrives, or a workload pinned to revision 2 would
// quietly come to mean something else.
func numberEnvironmentRevisions(item *environmentItem) {
	sort.SliceStable(item.Revisions, func(i, j int) bool {
		return item.Revisions[i].CreatedAt.Before(item.Revisions[j].CreatedAt)
	})
	for index := range item.Revisions {
		item.Revisions[index].Number = index + 1
	}
	sort.SliceStable(item.Revisions, func(i, j int) bool {
		return item.Revisions[i].CreatedAt.After(item.Revisions[j].CreatedAt)
	})
}

func addSystemEnvironment(items map[string]*environmentItem, projectID, image string, definition systemEnvironmentDefinition) {
	image = strings.TrimSpace(image)
	if image == "" {
		return
	}
	// The same key builds use: the repository, not the reference. Keying this
	// one on the full image while builds keyed on the repository produced
	// three rows for noryx-vscode:0.1.2 - the system definition, and the
	// rebuilds of it, none of them merging.
	// Never the project: a system environment belongs to the platform, and
	// keying it under whichever project happened to ask for the list would
	// hide it from the next one.
	key := "|" + imageRepository(image)
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
		// The platform's own image wins, and so does its name.
		//
		// Entries are keyed on the repository rather than the reference, so a
		// rebuild does not become a second environment - which is right, and
		// which let a rebuild quietly become the offered one. A build in a
		// project pushed noryx-vscode:1788812980 to the platform's repository;
		// the merged entry kept the build's image while showing the system
		// name, so the list read 0.1.2 and the launch carried the timestamp.
		// The platform runs what it is configured to run, and refused an image
		// nobody could see had been substituted.
		//
		// The rebuild is not lost: it stays in the revisions below, which is
		// where the history of an environment belongs. What it may not do is
		// replace what the platform will actually start.
		item.Name = deriveEnvironmentName(image)
		item.DestinationImage = image
		item.WorkspaceIDEs = mergeWorkspaceIDEs(definition.WorkspaceIDEs, item.WorkspaceIDEs)
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

// The order is the order the interface offers, and its first entry is what a
// launch uses when nobody chooses: VS Code comes first because it is what most
// people here open, and a default that matches the common case saves a click
// on every workspace.
func deriveWorkspaceIDEs(values ...string) []string {
	joined := strings.ToLower(strings.Join(values, "\n"))
	ides := []string{}
	if strings.Contains(joined, "noryx-python") || strings.Contains(joined, "vscode") || strings.Contains(joined, "openvscode") {
		ides = append(ides, "vscode")
	}
	if strings.Contains(joined, "noryx-python") || strings.Contains(joined, "jupyter") {
		ides = append(ides, "jupyter")
	}
	if strings.Contains(joined, "rstudio") || strings.Contains(joined, "rocker/") {
		ides = append(ides, "rstudio")
	}
	// And whatever this build registered. Without this a kind works through
	// the API and is offered nowhere: the environment screen decides what can
	// be launched, and it decides from the image name.
	for _, kind := range workspacekind.All() {
		if kind.Matches(values...) {
			ides = append(ides, kind.ID)
		}
	}
	return ides
}

func mergeWorkspaceIDEs(current, extra []string) []string {
	seen := map[string]bool{}
	for _, ide := range append(current, extra...) {
		ide = strings.ToLower(strings.TrimSpace(ide))
		if allowedWorkspaceIDEs[ide] || workspacekind.Allowed(ide) {
			seen[ide] = true
		}
	}
	// The order the caller gave, kept.
	//
	// This used to rebuild the list as jupyter, vscode, rstudio, then the
	// registered kinds - a fixed order meant to keep an interface's default
	// stable when a module is installed. The cost was that the first entry
	// stopped describing the environment: a VS Code environment that merged
	// with a rebuild came back jupyter-first, the launch form read entry zero,
	// and it offered JupyterLab under a name reading noryx-vscode. A list
	// whose order means nothing is a list nobody should read positionally, and
	// this one is read positionally.
	//
	// Callers pass the definitive kind first, so the stability that ordering
	// was protecting now comes from the caller knowing what this environment
	// is - which it does, and the sort never did.
	result := make([]string, 0, len(seen))
	added := map[string]bool{}
	for _, ide := range append(append([]string{}, current...), extra...) {
		ide = strings.ToLower(strings.TrimSpace(ide))
		if seen[ide] && !added[ide] {
			added[ide] = true
			result = append(result, ide)
		}
	}
	return result
}

func (h Handlers) workspaceEnvironmentAllowed(projectID, image, ide string) bool {
	image = strings.TrimSpace(image)
	ide = strings.ToLower(strings.TrimSpace(ide))
	if image == "" || (!allowedWorkspaceIDEs[ide] && !workspacekind.Allowed(ide)) {
		return false
	}
	if (ide == "jupyter" && image == strings.TrimSpace(h.workspaceJupyterImage)) ||
		(ide == "vscode" && image == strings.TrimSpace(h.workspaceVSCodeImage)) ||
		(ide == "rstudio" && image == strings.TrimSpace(h.workspaceRStudioImage)) {
		return true
	}
	// A registered kind's own image is its platform default, exactly as the
	// three above are theirs. Without this the kind is accepted everywhere else
	// and refused here, with a message blaming the environment for a rule it
	// never broke.
	if registered, ok := workspacekind.Lookup(ide); ok && registered.DefaultImage != nil {
		if configured := strings.TrimSpace(registered.DefaultImage()); configured != "" && configured == image {
			return true
		}
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

// systemEnvironmentRepositories is the set of repositories this platform ships,
// including those a registered kind contributes. Used to recognise a build that
// is a rebuild of one of them rather than a project's own image.
func (h Handlers) systemEnvironmentRepositories() map[string]bool {
	repositories := map[string]bool{}
	for _, image := range []string{h.workspaceJupyterImage, h.workspaceVSCodeImage, h.workspaceRStudioImage} {
		if repository := imageRepository(strings.TrimSpace(image)); repository != "" {
			repositories[repository] = true
		}
	}
	for _, kind := range workspacekind.All() {
		if kind.DefaultImage == nil {
			continue
		}
		if repository := imageRepository(strings.TrimSpace(kind.DefaultImage())); repository != "" {
			repositories[repository] = true
		}
	}
	return repositories
}

// deriveIDEForImage names the kind an image runs.
//
// The platform's three images answer for themselves, a registered kind answers
// for its own, and anything else falls back to what the image and its build
// say about themselves - the same derivation the environment list publishes,
// so the kind a launch gets is the kind the catalogue showed.
//
// Empty when nothing matches, which the caller reads as "no opinion" rather
// than as a kind.
func (h Handlers) deriveIDEForImage(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return ""
	}
	switch image {
	case strings.TrimSpace(h.workspaceVSCodeImage):
		return "vscode"
	case strings.TrimSpace(h.workspaceJupyterImage):
		return "jupyter"
	case strings.TrimSpace(h.workspaceRStudioImage):
		return "rstudio"
	}
	for _, kind := range workspacekind.All() {
		if kind.DefaultImage == nil {
			continue
		}
		if configured := strings.TrimSpace(kind.DefaultImage()); configured != "" && configured == image {
			return kind.ID
		}
	}
	if ides := deriveWorkspaceIDEs(image); len(ides) > 0 {
		return ides[0]
	}
	return ""
}

// configuredImageFor names what this platform runs for a kind, so a refusal can
// show the difference rather than assert one.
func (h Handlers) configuredImageFor(ide string) string {
	switch strings.ToLower(strings.TrimSpace(ide)) {
	case "vscode":
		return strings.TrimSpace(h.workspaceVSCodeImage)
	case "jupyter":
		return strings.TrimSpace(h.workspaceJupyterImage)
	case "rstudio":
		return strings.TrimSpace(h.workspaceRStudioImage)
	}
	if registered, ok := workspacekind.Lookup(ide); ok && registered.DefaultImage != nil {
		return strings.TrimSpace(registered.DefaultImage())
	}
	return ""
}

// reservedRepositoryFor names the platform repository an image would land in,
// or empty when it lands somewhere a project is free to use.
//
// Compared on the repository and never on the reference: the harm comes from
// sharing a repository, because that is what the catalogue keys entries on.
func (h Handlers) reservedRepositoryFor(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return ""
	}
	target := imageRepository(image)
	reserved := []string{h.workspaceVSCodeImage, h.workspaceJupyterImage, h.workspaceRStudioImage}
	for _, kind := range workspacekind.All() {
		if kind.DefaultImage != nil {
			reserved = append(reserved, kind.DefaultImage())
		}
	}
	for _, candidate := range reserved {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if imageRepository(candidate) == target {
			return target
		}
	}
	return ""
}
