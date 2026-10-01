package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
)

// Extracts: a named selection of files, frozen when it is declared.
//
// The question a study starts from is "the subjects with a corneal wavefront,
// first visit" - and the honest way to answer it twice is to resolve it once
// and keep the list. An extract that re-ran its filter would return a different
// study every month, which is how an n stops being reproducible.
//
// Nothing is copied. The members are keys in the dataset where the data already
// lives; a mount builds a tree of links over them. The source bucket is read
// only to the platform, and stays that way.

const extractMemberPageSize = 500

type extractRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ProjectID   string   `json:"projectId"`
	Subjects    []string `json:"subjects"`
	Modalities  []string `json:"modalities"`
	Visits      []string `json:"visits"`
}

func (h Handlers) CreateExtract(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	if h.extractStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "extracts require a persistent store"})
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

	var req extractRequest
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
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to resolve the extract"})
		return
	}
	// An ontology scanned before the platform kept its file list resolves to
	// nothing, and the fix is a rescan, not an extract of zero files that looks
	// like a legitimate empty result.
	if len(members) == 0 {
		stored, countErr := h.ontologyStore.CountObjects(ontologyID)
		if countErr == nil && stored == 0 {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "this ontology was scanned before file paths were kept; rescan it to build extracts from it"})
			return
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no object matches this selection"})
		return
	}

	// An extract has to know which project will mount it. The catalogue screen is
	// not project-scoped, so an ontology attached to exactly one project answers
	// for itself; anything else is asked rather than guessed, because an extract
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

	object := extractdomain.New(identity.UserID(), ontologyID, projectID, req.Name, req.Description, req.Subjects, req.Modalities, req.Visits)
	frozen := make([]extractdomain.Member, 0, len(members))
	for _, member := range members {
		object.TotalBytes += member.SizeBytes
		frozen = append(frozen, extractdomain.Member{
			ExtractID: object.ID,
			Path:      member.Path,
			SubjectID: member.SubjectID,
			Visit:     member.Visit,
			Modality:  member.Modality,
			SizeBytes: member.SizeBytes,
		})
	}
	object.ObjectCount = len(frozen)

	if err := h.extractStore.Create(object, frozen); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create the extract"})
		return
	}
	h.emitAudit(r, identity.UserID(), "extract.create", "extract", object.ID, object.ProjectID, "success", "", map[string]any{
		"ontologyId":  ontologyID,
		"objectCount": object.ObjectCount,
	})
	writeJSON(w, http.StatusCreated, object)
}

func (h Handlers) ListOntologyExtracts(w http.ResponseWriter, r *http.Request) {
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
	if h.extractStore == nil {
		writeJSON(w, http.StatusOK, []extractdomain.Extract{})
		return
	}
	items, err := h.extractStore.ListByOntology(ontologyID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list extracts"})
		return
	}
	writeJSON(w, http.StatusOK, items)
}

// GetExtractMembers is the frozen list itself: what a mount reads, and what a
// reviewer asks for when they want to know which files an n was computed over.
func (h Handlers) GetExtractMembers(w http.ResponseWriter, r *http.Request) {
	identity, item, ok := h.requireExtract(w, r)
	if !ok {
		return
	}
	limit := extractMemberPageSize
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	members, err := h.extractStore.ListMembers(item.ID, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list the extract's files"})
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
		"extract":  item,
		"members":  members,
		"subjects": names,
		// The page, not the extract: a reader must not mistake 500 shown files
		// for the size of the study.
		"shown": len(members),
		"total": item.ObjectCount,
	})
}

func (h Handlers) DeleteExtract(w http.ResponseWriter, r *http.Request) {
	identity, item, ok := h.requireExtract(w, r)
	if !ok {
		return
	}
	if !h.canManageExtract(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "only the extract's owner can delete it"})
		return
	}
	if err := h.extractStore.Delete(item.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete the extract"})
		return
	}
	h.emitAudit(r, identity.UserID(), "extract.delete", "extract", item.ID, item.ProjectID, "success", "", nil)
	w.WriteHeader(http.StatusNoContent)
}

// An extract is visible to whoever can read the ontology it came from: it names
// files in that ontology's source and nothing else.
func (h Handlers) requireExtract(w http.ResponseWriter, r *http.Request) (identity auth.Identity, item extractdomain.Extract, ok bool) {
	resolved, authorised := h.requireIdentity(w, r)
	if !authorised {
		return identity, item, false
	}
	if h.extractStore == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "extract not found"})
		return identity, item, false
	}
	found := false
	var err error
	item, found, err = h.extractStore.GetByID(strings.TrimSpace(r.PathValue("extractID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "extract not found"})
		return identity, item, false
	}
	ontology, foundOntology, err := h.ontologyStore.GetByID(item.OntologyID)
	if err != nil || !foundOntology {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "extract not found"})
		return identity, item, false
	}
	if !h.isGlobalAdmin(resolved) && h.ontologyRole(ontology, resolved) == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "extract not found"})
		return identity, item, false
	}
	return resolved, item, true
}

// canManageExtract reports whether this person may hand the extract on, or
// delete it.
//
// Read from the owner, not from the author. The check used to compare
// OwnerUserID - whoever declared the extract - which was the same thing while
// an extract could not be transferred. The moment it can, the two part company:
// hand an extract to an organization and its author would keep the right to
// delete it while a member of the owning organization would not.
//
// The author keeps the extract only until somebody moves it, which is exactly
// what the default owner says.
func (h Handlers) canManageExtract(item extractdomain.Extract, identity auth.Identity) bool {
	if h.isGlobalAdmin(identity) {
		return true
	}
	for _, subject := range h.ontologySubjects(identity) {
		if strings.EqualFold(item.OwnerType, subject.Type) && strings.EqualFold(item.OwnerID, subject.ID) {
			return true
		}
	}
	// Les lignes anterieures a la migration n ont pas de proprietaire : leur
	// auteur garde la main plutot que personne.
	return strings.TrimSpace(item.OwnerType) == "" && item.OwnerUserID == identity.UserID()
}

// UpdateExtractOwner hands an extract to a person or an organization, in the
// same shape and the same words as a dataset or an ontology.
func (h Handlers) UpdateExtractOwner(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.extractStore.GetByID(strings.TrimSpace(r.PathValue("extractID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "extract not found"})
		return
	}
	if !h.canManageExtract(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "extract owner or global admin required"})
		return
	}
	var req setDatasetOwnerRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid ownerType and ownerId are required"})
		return
	}
	ownerType, ownerID, status, probleme := h.normaliseOwner(req.OwnerType, req.OwnerID, identity)
	if status != 0 {
		writeJSON(w, status, map[string]string{"error": probleme})
		return
	}
	req.OwnerType, req.OwnerID = ownerType, ownerID
	if err := h.extractStore.SetOwner(item.ID, req.OwnerType, req.OwnerID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to transfer the extract"})
		return
	}
	h.emitAudit(r, identity.UserID(), "extract.owner.changed", "extract", item.ID, item.ProjectID,
		"success", "", map[string]any{"ownerType": req.OwnerType, "ownerId": req.OwnerID,
			"previousOwnerType": item.OwnerType, "previousOwnerId": item.OwnerID})
	updated, _, _ := h.extractStore.GetByID(item.ID)
	writeJSON(w, http.StatusOK, updated)
}
