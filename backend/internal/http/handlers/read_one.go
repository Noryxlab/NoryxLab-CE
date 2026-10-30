package handlers

import (
	"net/http"
	"strings"

	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	datasourcedomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/datasource"
	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
)

// Reading one object by its identifier.
//
// The API was list-shaped: every screen that shows one object asks for the
// collection and picks, which is why nothing in the interface missed these.
// An integration had to do the same - fetch every dataset to look at one -
// and had no way to ask the only question that matters about an identifier
// somebody pasted: may I see this?
//
// Found on 2026-10-30 by a rights sweep that tripped on it three times
// before believing it: a `GET` on a named object answered 404 from the
// router, which is indistinguishable from a refusal and sent the reader
// hunting for a wrong identifier.
//
// Each handler answers the same question its listing answers, with the same
// helper, so the two can never disagree: an object absent from your listing
// is absent here. And the refusal is **not found, never forbidden** - naming
// an identifier must not confirm that it exists, which is the rule the rest
// of the platform already follows.
//
// An environment has no handler here, and that is not an omission: it is not
// a stored object but a view computed over builds, keyed by the image its
// revisions push to. Reading one means filtering that computed list, which is
// what the screen does.

// GetDataset answers with one dataset, as the catalogue sees it.
func (h Handlers) GetDataset(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the dataset"})
		return
	}
	if !found || !h.canSeeDataset(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}
	named := []datasetdomain.Dataset{item}
	h.nameDatasetOwners(named)
	writeJSON(w, http.StatusOK, named[0])
}

// GetOntology answers with one ontology.
func (h Handlers) GetOntology(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.ontologyStore.GetByID(strings.TrimSpace(r.PathValue("ontologyID")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the ontology"})
		return
	}
	if !found || !h.canSeeOntology(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}
	named := []ontologydomain.Ontology{item}
	h.nameOntologyOwners(named)
	writeJSON(w, http.StatusOK, named[0])
}

// GetExtract answers with one extract. A reader of the ontology it is drawn
// from sees it too, which is what the listing does.
func (h Handlers) GetExtract(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.extractStore.GetByID(strings.TrimSpace(r.PathValue("extractID")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the extract"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "extract not found"})
		return
	}
	ontology, _, _ := h.ontologyStore.GetByID(item.OntologyID)
	if !h.canReadExtract(item, ontology, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "extract not found"})
		return
	}
	named := []extractdomain.Extract{item}
	h.nameExtractOwners(named)
	writeJSON(w, http.StatusOK, named[0])
}

// GetDatasource answers with one connector, without its password: the secret
// is reached by its own route, under its own decision.
func (h Handlers) GetDatasource(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasourceStore.GetByID(strings.TrimSpace(r.PathValue("datasourceID")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the datasource"})
		return
	}
	if !found || !h.canSeeDatasource(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "datasource not found"})
		return
	}
	named := []datasourcedomain.Datasource{item}
	h.nameDatasourceOwners(named)
	writeJSON(w, http.StatusOK, named[0])
}

// GetRepository answers with one repository. Repositories are not shared, so
// the owner and a platform administrator see one.
func (h Handlers) GetRepository(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.repositoryStore.GetByID(strings.TrimSpace(r.PathValue("repositoryID")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the repository"})
		return
	}
	if !found || !(h.isGlobalAdmin(identity) || strings.EqualFold(strings.TrimSpace(item.OwnerUserID), identity.UserID())) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "repository not found"})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// GetProject answers with one project, carrying the caller's own role on it
// and whether they may transfer it - the two fields the listing adds, and the
// two a screen needs to decide what to offer.
func (h Handlers) GetProject(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	item, found, err := h.projectByID(strings.TrimSpace(r.PathValue("projectID")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the project"})
		return
	}
	if !found || !h.hasProjectMembership(userID, item.ID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	role, _ := h.effectiveProjectRole(item.ID, userID)
	item.Role = string(role)
	item.CanManageOwner = h.isGlobalAdminUserID(userID) || h.projectOwnedBy(item, userID)
	named := []project.Project{item}
	h.nameProjectOwners(named)
	writeJSON(w, http.StatusOK, named[0])
}

// GetApp answers with one app, API or dashboard, with its runtime status
// resolved as the listing resolves it.
func (h Handlers) GetApp(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	item, found, err := h.appStore.GetByID(strings.TrimSpace(r.PathValue("appID")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the app"})
		return
	}
	if !found || !h.hasProjectMembership(userID, item.ProjectID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}
	writeJSON(w, http.StatusOK, h.enrichAppRuntimeStatus(item))
}

// GetJob answers with one job.
func (h Handlers) GetJob(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	h.syncJobsFromRuntime()
	item, found, err := h.jobStore.GetByID(strings.TrimSpace(r.PathValue("jobID")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the job"})
		return
	}
	if !found || !h.hasProjectMembership(userID, item.ProjectID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// GetWorkspace answers with one workspace.
func (h Handlers) GetWorkspace(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	h.syncWorkspacesFromRuntime(userID)
	item, found, err := h.workspaceStore.GetByID(strings.TrimSpace(r.PathValue("workspaceID")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the workspace"})
		return
	}
	if !found || !h.hasProjectMembership(userID, item.ProjectID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "workspace not found"})
		return
	}
	writeJSON(w, http.StatusOK, item)
}

// GetBuild answers with one build, its failure reason included.
func (h Handlers) GetBuild(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	h.syncBuildsFromRuntime()
	item, found, err := h.buildStore.GetByID(strings.TrimSpace(r.PathValue("buildID")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the build"})
		return
	}
	if !found || !h.hasProjectMembership(userID, item.ProjectID) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "build not found"})
		return
	}
	writeJSON(w, http.StatusOK, item)
}
