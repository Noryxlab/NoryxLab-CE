package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/build"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

type createBuildRequest struct {
	ProjectID string `json:"projectId"`
	// Name is what the interface sends when somebody writes a Dockerfile in
	// the environment screen: there is no repository to clone and no registry
	// address a browser could know, so the platform derives both.
	Name              string `json:"name"`
	GitRepository     string `json:"gitRepository"`
	GitRef            string `json:"gitRef"`
	DockerfilePath    string `json:"dockerfilePath"`
	DockerfileContent string `json:"dockerfileContent"`
	ContextPath       string `json:"contextPath"`
	DestinationImage  string `json:"destinationImage"`
}

func (h Handlers) ListBuilds(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	h.syncBuildsFromRuntime()

	items, err := h.buildStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list builds"})
		return
	}

	projectFilter := strings.TrimSpace(r.URL.Query().Get("projectId"))
	filtered := make([]build.Build, 0, len(items))
	for _, item := range items {
		if projectFilter != "" && item.ProjectID != projectFilter {
			continue
		}
		if !h.hasProjectMembership(userID, item.ProjectID) {
			continue
		}
		filtered = append(filtered, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": filtered})
}

func (h Handlers) CreateBuild(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	var req createBuildRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.GitRepository = strings.TrimSpace(req.GitRepository)
	req.DestinationImage = strings.TrimSpace(req.DestinationImage)
	req.DockerfileContent = strings.TrimSpace(req.DockerfileContent)
	req.Name = strings.TrimSpace(req.Name)

	// A build has to come from somewhere and go somewhere. There are two ways
	// to say it, and only one of them was accepted:
	//
	//   - a repository to clone and an image to push, which is what a build
	//     driven by an API client sends;
	//   - a Dockerfile written in the environment screen, where there is no
	//     repository at all and where a browser cannot know the registry the
	//     platform pushes to.
	//
	// The second was refused with "projectId, gitRepository and
	// destinationImage are required" - a message naming three fields, two of
	// which the person had no way to provide. So the platform now fills them
	// in: the Dockerfile is the source, and the destination is derived from
	// the registry this installation already pulls its own environments from.
	if req.ProjectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectId is required"})
		return
	}
	if req.GitRepository == "" && req.DockerfileContent == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "a build needs either a gitRepository to clone or a dockerfileContent to build",
		})
		return
	}
	// A project may not build into a repository the platform runs from.
	//
	// Derived destinations are already safe - they carry the project in the
	// repository name - but a caller may name one, and on 2026-09-07 one named
	// the platform's own noryx-vscode repository. Catalogue entries are keyed
	// on the repository, so that build merged with the system environment and
	// became the image the list offered, while the platform went on running
	// the tag it is configured with. Every launch from that entry was refused
	// for an image nobody could see had been substituted.
	//
	// Refused rather than allowed-and-ignored: building there cannot achieve
	// anything, since the platform starts the reference it was configured with
	// and not whatever was last pushed beside it. A build that cannot have an
	// effect should fail where it is asked for, not where it is used.
	if reserved := h.reservedRepositoryFor(req.DestinationImage); reserved != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error": "the repository " + reserved + " belongs to the platform's own " +
				"environments; build to a name of your own and the platform will offer it beside them",
		})
		return
	}
	if req.DestinationImage == "" {
		derived, err := h.deriveEnvironmentImage(req.ProjectID, req.Name)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		req.DestinationImage = derived
	}
	// The tag is the revision number, and `latest` follows the newest build.
	// A workload that pins a revision keeps the image it was built against;
	// one that does not gets the current environment. Both are pushed in the
	// same pass, so the second tag costs a manifest and not a copy of the
	// layers - which is the whole reason this is affordable.
	extraDestinations := []string(nil)
	if imageRepository(req.DestinationImage) == req.DestinationImage {
		repository := req.DestinationImage
		req.DestinationImage = repository + ":" + h.nextRevisionTag(req.ProjectID, repository)
		extraDestinations = append(extraDestinations, repository+":latest")
	}
	if req.DockerfilePath == "" {
		req.DockerfilePath = "Dockerfile"
	}

	exists, err := h.projectExists(req.ProjectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to verify project"})
		return
	}
	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	if !h.requireProjectRole(w, req.ProjectID, userID, actionRunBuild, "build submission") {
		return
	}

	jobName := "build-" + shortID()
	record := build.New(
		req.ProjectID,
		req.GitRepository,
		req.GitRef,
		req.DockerfilePath,
		req.ContextPath,
		req.DestinationImage,
		jobName,
	)
	record.DockerfileContent = req.DockerfileContent
	record.Name = req.Name
	// Resolved now, not later: `main` points somewhere today and somewhere
	// else next week, so the commit has to be captured at the moment the build
	// is asked for or it cannot be recovered at all. Empty when the repository
	// cannot be read anonymously, and empty reads as unknown.
	record.CommitSHA = resolveCommitSHA(req.GitRepository, req.GitRef)

	if h.runtime != nil {
		err = h.runtime.CreateBuild(noryxruntime.BuildSpec{
			JobName:            jobName,
			ContextGitURL:      req.GitRepository,
			GitRef:             req.GitRef,
			DockerfilePath:     req.DockerfilePath,
			DockerfileContent:  req.DockerfileContent,
			ContextPath:        req.ContextPath,
			DestinationImage:   req.DestinationImage,
			ExtraDestinations:  extraDestinations,
			PullSecret:         h.registryPullSecret,
			RegistrySecretName: h.registryPushSecret,
			Labels: map[string]string{
				"app.kubernetes.io/name": "noryx-build",
				"noryx.io/project-id":    req.ProjectID,
				"noryx.io/build-id":      record.ID,
			},
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes build submission failed: " + err.Error()})
			return
		}
	}

	if err := h.buildStore.Create(record); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save build"})
		return
	}

	h.emitAudit(r, userID, "environment.build.submit", "build", record.ID, record.ProjectID, "success", "", map[string]any{
		"destinationImage": record.DestinationImage,
		"gitRepository":    record.GitRepository,
		"gitRef":           record.GitRef,
		"dockerfilePath":   record.DockerfilePath,
		"inlineDockerfile": strings.TrimSpace(record.DockerfileContent) != "",
	})
	writeJSON(w, http.StatusCreated, record)
}

func (h Handlers) syncBuildsFromRuntime() {
	discovery, ok := h.runtime.(noryxruntime.BuildDiscovery)
	if !ok {
		return
	}
	runtimeItems, err := discovery.ListBuilds()
	if err != nil {
		return
	}

	seen := make(map[string]struct{}, len(runtimeItems))
	for _, item := range runtimeItems {
		buildID := strings.TrimSpace(item.BuildID)
		projectID := strings.TrimSpace(item.ProjectID)
		if buildID == "" || projectID == "" {
			continue
		}
		seen[buildID] = struct{}{}
		h.ensureProjectInStore(projectID)

		existing, found, err := h.buildStore.GetByID(buildID)
		if err != nil {
			continue
		}

		record := build.Build{
			ID:               buildID,
			ProjectID:        projectID,
			GitRepository:    strings.TrimSpace(item.GitRepository),
			GitRef:           strings.TrimSpace(item.GitRef),
			DockerfilePath:   strings.TrimSpace(item.DockerfilePath),
			ContextPath:      strings.TrimSpace(item.ContextPath),
			DestinationImage: strings.TrimSpace(item.DestinationImage),
			JobName:          strings.TrimSpace(item.JobName),
			Status:           strings.TrimSpace(item.Status),
			CreatedAt:        time.Now().UTC(),
		}

		if found {
			record = existing
			record.ProjectID = projectID
			if v := strings.TrimSpace(item.GitRepository); v != "" {
				record.GitRepository = v
			}
			if v := strings.TrimSpace(item.GitRef); v != "" {
				record.GitRef = v
			}
			if v := strings.TrimSpace(item.DockerfilePath); v != "" {
				record.DockerfilePath = v
			}
			if v := strings.TrimSpace(item.ContextPath); v != "" {
				record.ContextPath = v
			}
			if v := strings.TrimSpace(item.DestinationImage); v != "" {
				record.DestinationImage = v
			}
			if v := strings.TrimSpace(item.JobName); v != "" {
				record.JobName = v
			}
			if v := strings.TrimSpace(item.Status); v != "" {
				record.Status = v
			}
			if record.CreatedAt.IsZero() {
				record.CreatedAt = time.Now().UTC()
			}
		} else if record.Status == "" {
			record.Status = "submitted"
		}

		// The digest is resolved once, when the build has actually pushed
		// something, and never again: a tag can be moved afterwards, and the
		// image this build produced is the one that existed when it finished.
		// Guarded on an empty value rather than on the status alone, so a
		// sweep over a hundred finished builds does not become a hundred
		// registry calls every time it runs.
		if record.ImageDigest == "" && strings.EqualFold(strings.TrimSpace(record.Status), "succeeded") {
			record.ImageDigest = h.resolveImageDigest(record.DestinationImage)
		}

		_ = h.buildStore.Upsert(record)
	}

	// Reconcile stale in-store builds that disappeared from runtime jobs.
	// Typical case: jobs manually deleted from Kubernetes while DB still says running.
	existing, err := h.buildStore.List()
	if err != nil {
		return
	}
	for _, item := range existing {
		id := strings.TrimSpace(item.ID)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		status := strings.ToLower(strings.TrimSpace(item.Status))
		if status == "running" || status == "submitted" {
			item.Status = "canceled"
			_ = h.buildStore.Upsert(item)
		}
	}
}

// deriveEnvironmentImage answers with where a custom environment's image goes.
//
// The registry is taken from the images this installation already runs - the
// platform pulls noryx-vscode from it, so it is the registry this cluster can
// reach and the one the build's push secret is for. Nothing here is a name a
// browser could have invented, which is why the interface was never able to
// send it.
//
// The tag is the project and the environment, so two projects building an
// environment called "training" do not overwrite each other's image, and so
// somebody reading a workload's image can tell whose environment it is.
func (h Handlers) deriveEnvironmentImage(projectID, name string) (string, error) {
	slug := environmentSlug(name)
	if slug == "" {
		return "", fmt.Errorf("a name is required to build an environment")
	}
	reference := strings.TrimSpace(h.workspaceVSCodeImage)
	if reference == "" {
		reference = strings.TrimSpace(h.workspaceJupyterImage)
	}
	if reference == "" {
		return "", fmt.Errorf("this installation has no environment registry configured")
	}
	repository, _, _ := strings.Cut(reference, ":")
	registryProject := path.Dir(repository)
	if registryProject == "." || registryProject == "/" {
		return "", fmt.Errorf("this installation has no environment registry configured")
	}
	// A project of its own, beside the platform's.
	//
	// Until 2026-09-18 a derived destination landed in the same registry
	// project as the images the platform runs from, separated only by a name
	// prefix. Nothing but that prefix stood between a build and a repository
	// the platform starts workspaces from - and on 2026-09-07 a build reached
	// one, merged with the system entry in the catalogue, and broke every
	// launch from it. The guard added that day refuses the collision; this
	// removes the adjacency that made it possible, and takes the registry's
	// own access control with it: a robot that may push here need not be able
	// to push there.
	registryProject = buildRegistryProject(registryProject, h.buildRegistryProject)
	project := environmentSlug(projectID)
	if len(project) > 12 {
		project = project[:12]
	}
	return fmt.Sprintf("%s/%s-%s", registryProject, project, slug), nil
}

// buildRegistryProject names the registry project derived builds are pushed
// to. Configured outright, or a sibling of the platform's own project - never
// the platform's project itself, which is the arrangement this replaces.
func buildRegistryProject(platformProject, configured string) string {
	if configured = strings.TrimSpace(configured); configured != "" {
		return configured
	}
	registry, project := path.Split(platformProject)
	if project == "" {
		return platformProject
	}
	return path.Join(registry, project+"-builds")
}

// nextRevisionTag counts the revisions this environment already has and names
// the next one. `r3` rather than a Unix timestamp: it is what the screen shows
// and what somebody says out loud, and it sorts the way a person expects.
func (h Handlers) nextRevisionTag(projectID, repository string) string {
	revisions := 0
	if builds, err := h.buildStore.List(); err == nil {
		for _, item := range builds {
			if item.ProjectID == projectID && imageRepository(strings.TrimSpace(item.DestinationImage)) == repository {
				revisions++
			}
		}
	}
	return fmt.Sprintf("r%d", revisions+1)
}

// environmentSlug reduces a name to what a registry accepts in a repository
// path: lower case, digits, dashes.
func environmentSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var out strings.Builder
	previousDash := false
	for _, r := range value {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			out.WriteRune(r)
			previousDash = false
		case out.Len() > 0 && !previousDash:
			out.WriteRune('-')
			previousDash = true
		}
	}
	return strings.Trim(out.String(), "-")
}
