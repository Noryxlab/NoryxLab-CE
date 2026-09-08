package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	cohortdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/cohort"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
)

// Cohorts: a named selection of files, frozen when it is declared.
//
// The question a study starts from is "the subjects with a corneal wavefront,
// first visit" - and the honest way to answer it twice is to resolve it once
// and keep the list. A cohort that re-ran its filter would return a different
// study every month, which is how an n stops being reproducible.
//
// Nothing is copied. The members are keys in the dataset where the data already
// lives; a mount builds a tree of links over them. The source bucket is read
// only to the platform, and stays that way.

const cohortMemberPageSize = 500

type cohortRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ProjectID   string   `json:"projectId"`
	Subjects    []string `json:"subjects"`
	Modalities  []string `json:"modalities"`
	Visits      []string `json:"visits"`
}

func (h Handlers) CreateCohort(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	if h.cohortStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "cohorts require a persistent store"})
		return
	}
	ontologyID := strings.TrimSpace(r.PathValue("ontologyID"))
	item, found, err := h.ontologyStore.GetByID(ontologyID)
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}
	if !h.isGlobalAdmin(identity) && h.ontologyRole(item, identity) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}

	var req cohortRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	members, err := h.ontologyStore.ListObjects(ontologyID, ontologydomain.ObjectFilter{
		Subjects:   req.Subjects,
		Modalities: req.Modalities,
		Visits:     req.Visits,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to resolve the cohort"})
		return
	}
	// An ontology scanned before the platform kept its file list resolves to
	// nothing, and the fix is a rescan, not a cohort of zero files that looks
	// like a legitimate empty result.
	if len(members) == 0 {
		stored, countErr := h.ontologyStore.CountObjects(ontologyID)
		if countErr == nil && stored == 0 {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "this ontology was scanned before file paths were kept; rescan it to build cohorts from it"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no object matches this selection"})
		return
	}

	// A cohort has to know which project will mount it. The catalogue screen is
	// not project-scoped, so an ontology attached to exactly one project answers
	// for itself; anything else is asked rather than guessed, because a cohort
	// filed under the wrong project would simply never appear in a workspace.
	projectID := strings.TrimSpace(req.ProjectID)
	if projectID == "" && h.projectResourceStore != nil {
		linked, err := h.projectResourceStore.ListOntologyProjectIDs(ontologyID)
		if err == nil && len(linked) == 1 {
			projectID = linked[0]
		}
	}
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectId is required: this ontology is attached to several projects, or to none"})
		return
	}

	object := cohortdomain.New(identity.UserID(), ontologyID, projectID, req.Name, req.Description, req.Subjects, req.Modalities, req.Visits)
	frozen := make([]cohortdomain.Member, 0, len(members))
	for _, member := range members {
		object.TotalBytes += member.SizeBytes
		frozen = append(frozen, cohortdomain.Member{
			CohortID:  object.ID,
			Path:      member.Path,
			SubjectID: member.SubjectID,
			Visit:     member.Visit,
			Modality:  member.Modality,
			SizeBytes: member.SizeBytes,
		})
	}
	object.ObjectCount = len(frozen)

	if err := h.cohortStore.Create(object, frozen); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create the cohort"})
		return
	}
	h.emitAudit(r, identity.UserID(), "cohort.create", "cohort", object.ID, object.ProjectID, "success", "", map[string]any{
		"ontologyId":  ontologyID,
		"objectCount": object.ObjectCount,
	})
	writeJSON(w, http.StatusCreated, object)
}

func (h Handlers) ListOntologyCohorts(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	ontologyID := strings.TrimSpace(r.PathValue("ontologyID"))
	item, found, err := h.ontologyStore.GetByID(ontologyID)
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}
	if !h.isGlobalAdmin(identity) && h.ontologyRole(item, identity) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}
	if h.cohortStore == nil {
		writeJSON(w, http.StatusOK, []cohortdomain.Cohort{})
		return
	}
	items, err := h.cohortStore.ListByOntology(ontologyID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list cohorts"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// GetCohortMembers is the frozen list itself: what a mount reads, and what a
// reviewer asks for when they want to know which files an n was computed over.
func (h Handlers) GetCohortMembers(w http.ResponseWriter, r *http.Request) {
	identity, item, ok := h.requireCohort(w, r)
	if !ok {
		return
	}
	limit := cohortMemberPageSize
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	members, err := h.cohortStore.ListMembers(item.ID, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list the cohort's files"})
		return
	}
	subjects := map[string]bool{}
	for _, member := range members {
		subjects[member.SubjectID] = true
	}
	names := make([]string, 0, len(subjects))
	for name := range subjects {
		names = append(names, name)
	}
	sort.Strings(names)
	_ = identity
	writeJSON(w, http.StatusOK, map[string]any{
		"cohort":   item,
		"members":  members,
		"subjects": names,
		// The page, not the cohort: a reader must not mistake 500 shown files
		// for the size of the study.
		"shown": len(members),
		"total": item.ObjectCount,
	})
}

func (h Handlers) DeleteCohort(w http.ResponseWriter, r *http.Request) {
	identity, item, ok := h.requireCohort(w, r)
	if !ok {
		return
	}
	if item.OwnerUserID != identity.UserID() && !h.isGlobalAdmin(identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "only the cohort's owner can delete it"})
		return
	}
	if err := h.cohortStore.Delete(item.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete the cohort"})
		return
	}
	h.emitAudit(r, identity.UserID(), "cohort.delete", "cohort", item.ID, item.ProjectID, "success", "", nil)
	w.WriteHeader(http.StatusNoContent)
}

// A cohort is visible to whoever can read the ontology it came from: it names
// files in that ontology's source and nothing else.
func (h Handlers) requireCohort(w http.ResponseWriter, r *http.Request) (identity auth.Identity, item cohortdomain.Cohort, ok bool) {
	resolved, authorised := h.requireIdentity(w, r)
	if !authorised {
		return identity, item, false
	}
	if h.cohortStore == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "cohort not found"})
		return identity, item, false
	}
	found := false
	var err error
	item, found, err = h.cohortStore.GetByID(strings.TrimSpace(r.PathValue("cohortID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "cohort not found"})
		return identity, item, false
	}
	ontology, foundOntology, err := h.ontologyStore.GetByID(item.OntologyID)
	if err != nil || !foundOntology {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "cohort not found"})
		return identity, item, false
	}
	if !h.isGlobalAdmin(resolved) && h.ontologyRole(ontology, resolved) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "cohort not found"})
		return identity, item, false
	}
	return resolved, item, true
}
