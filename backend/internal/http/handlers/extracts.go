package handlers

import (
	"encoding/json"
	"log"
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
	// Layout is the order of the directory levels the mount builds. Empty is
	// the default, subject first, which is what every extract carried before
	// the order could be chosen.
	Layout []string `json:"layout"`
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

	// A project is optional, and attaching is a separate gesture.
	//
	// It used to be required: an extract that named no project was refused,
	// and the catalogue screen had to ask for one before it could freeze a
	// selection. But what an extract describes has nothing to do with which
	// projects mount it - that is a later question, and one with more than one
	// answer. Naming a project here simply attaches it straight away, which is
	// the common case and saves a second click.
	projectID := strings.TrimSpace(req.ProjectID)
	if projectID == "" && h.projectResourceStore != nil {
		// Attached to exactly one project, the ontology answers for itself.
		linked, err := h.projectResourceStore.ListOntologyProjectIDs(ontologyID)
		if err == nil && len(linked) == 1 {
			projectID = linked[0]
		}
	}

	layout, probleme := extractdomain.NormaliseLayout(req.Layout)
	if probleme != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": probleme})
		return
	}
	// A layout that names fewer than three levels merges what it leaves out,
	// and the mount builds links: two files on one path means one of them is
	// not there. Checked against the selection, because only the files can
	// say - dropping the visit is harmless on a study with one visit per
	// patient and destructive on the next study along.
	if conflit, collides := layoutCollision(layout, members); collides {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": conflit})
		return
	}

	object := extractdomain.New(identity.UserID(), ontologyID, projectID, req.Name, req.Description, req.Subjects, req.Modalities, req.Visits)
	object.Layout = layout
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
	if projectID != "" && h.projectResourceStore != nil {
		// Un echec de rattachement ne doit pas perdre l extrait : il existe,
		// il est gele, et il se rattache en un geste de plus.
		if err := h.projectResourceStore.AttachExtract(projectID, object.ID); err != nil {
			log.Printf("extract %s created but not attached to project %s: %v", object.ID, projectID, err)
		}
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
	h.nameExtractOwners(items)
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

// UpdateExtractMetadata corrects the label a person typed.
//
// The name is typed at declaration - "anterion-3-sujets" - and a typed name is
// a name that gets typed wrong. It could not be changed, so the only way to
// fix one was to declare the selection again: a different frozen list over a
// bucket that keeps growing, which means a rename produced a different study.
// The ontology got the same screen for the same reason on 2026-10-01.
//
// The label moves and nothing else does. The frozen file list, the author and
// the dates are what the extract is, and an n already published against this
// name still describes the same files.
func (h Handlers) UpdateExtractMetadata(w http.ResponseWriter, r *http.Request) {
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
	var req updateDatasetMetadataRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid name and description are required"})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	if err := h.extractStore.UpdateMetadata(item.ID, req.Name, req.Description); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to rename the extract"})
		return
	}
	h.emitAudit(r, identity.UserID(), "extract.renamed", "extract", item.ID, item.ProjectID,
		"success", "", map[string]any{"name": strings.TrimSpace(req.Name), "previousName": item.Name})
	updated, _, _ := h.extractStore.GetByID(item.ID)
	writeJSON(w, http.StatusOK, updated)
}

// AttachProjectExtract mounts an extract in a project's workspaces.
//
// The same gesture as attaching a dataset or an ontology, and for the same
// reason: what an extract describes has nothing to do with which projects use
// it. It used to be decided once, when the extract was declared, and never
// again - so an extract made in the wrong project simply never appeared.
func (h Handlers) AttachProjectExtract(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	extractID := strings.TrimSpace(r.PathValue("extractID"))
	if projectID == "" || extractID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID and extractID are required"})
		return
	}
	if !h.requireProjectRole(w, projectID, identity.UserID(), actionAttachOntology, "extract attach") {
		return
	}
	item, found, err := h.extractStore.GetByID(extractID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read extract"})
		return
	}
	// Not found rather than forbidden when it is not yours: naming an
	// identifier should not confirm that it exists.
	if !found || (!h.canManageExtract(item, identity) && !h.canReadOntologyObjectID(item.OntologyID, identity)) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "extract not found"})
		return
	}
	if err := h.projectResourceStore.AttachExtract(projectID, extractID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to attach the extract"})
		return
	}
	h.emitAudit(r, identity.UserID(), "extract.attached", "extract", extractID, projectID, "success", "", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "attached"})
}

// DetachProjectExtract stops mounting it. The extract itself is untouched: a
// frozen selection outlives the projects that used it.
func (h Handlers) DetachProjectExtract(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	extractID := strings.TrimSpace(r.PathValue("extractID"))
	if projectID == "" || extractID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID and extractID are required"})
		return
	}
	if !h.requireProjectRole(w, projectID, identity.UserID(), actionAttachOntology, "extract detach") {
		return
	}
	if err := h.projectResourceStore.DetachExtract(projectID, extractID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to detach the extract"})
		return
	}
	h.emitAudit(r, identity.UserID(), "extract.detached", "extract", extractID, projectID, "success", "", nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "detached"})
}

// ListProjectExtracts is what this project mounts.
//
// The screen that attaches needs to show what is already attached, and the
// link table is the only thing that knows: an extract no longer carries the
// project that mounts it.
func (h Handlers) ListProjectExtracts(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID is required"})
		return
	}
	if !h.requireProjectRole(w, projectID, identity.UserID(), actionRead, "extract list") {
		return
	}
	items, err := h.projectExtracts(projectID)
	if err != nil {
		h.nameExtractOwners(items)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read extracts"})
		return
	}
	h.nameExtractOwners(items)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// ListExtracts is the catalogue entry: every extract the caller can reach,
// without naming an ontology first.
//
// Visibility is the ontology's, not the extract's own. An extract is a frozen
// selection over one ontology's subjects, so somebody who cannot read the
// ontology has no business seeing a selection drawn from it - even one handed
// to their team. Ownership decides who may transfer or delete it; the ontology
// decides who may see that it exists.
func (h Handlers) ListExtracts(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	if h.extractStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []extractdomain.Extract{}})
		return
	}
	ontologies, err := h.ontologiesVisibleTo(identity)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list extracts"})
		return
	}
	items := []extractdomain.Extract{}
	for _, ontology := range ontologies {
		listed, err := h.extractStore.ListByOntology(ontology.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list extracts"})
			return
		}
		items = append(items, listed...)
	}
	h.nameExtractOwners(items)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
