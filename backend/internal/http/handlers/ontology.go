package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/datasource"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	"github.com/minio/minio-go/v7"
)

const ontologyScanMaxObjects = 50000

var (
	// A subject identifier, as the studies actually write them.
	//
	// The rule was one hyphen and a number: PREMYOM1000-0001 passed and
	// SELENA-01-001 did not, because the alphanumeric part cannot cross a
	// hyphen. FOR numbers SELENA patients by centre, so the whole string is
	// the patient - and a scan of that study recognised no subject at all,
	// producing an empty ontology that looked like it had worked.
	//
	// Segments before the number are now allowed. What is not relaxed is the
	// ending: a subject still finishes with three or four digits, which is
	// what keeps a directory like "DICOM" or "modality_ANTERION" from being
	// read as a patient.
	ontologySubjectPattern = regexp.MustCompile(`^[A-Za-z0-9]+(-[A-Za-z0-9]+)*-[0-9]{3,4}$`)
	ontologyDatePattern    = regexp.MustCompile(`^[0-9]{8}$`)
)

type ontologyScanRequest struct {
	DatasetID        string `json:"datasetId"`
	DatasourceID     string `json:"datasourceId"`
	SourceType       string `json:"sourceType"`
	InferenceProfile string `json:"inferenceProfile"`
}

type ontologyQueryRequest struct {
	Object    string `json:"object"`
	Type      string `json:"type"`
	Parent    string `json:"parent"`
	Attribute string `json:"attribute"`
	Reference string `json:"reference"`
	Limit     int    `json:"limit"`
}

type ontologyQueryItem struct {
	Object     string   `json:"object"`
	Type       string   `json:"type"`
	Parent     string   `json:"parent"`
	Attributes []string `json:"attributes"`
	References []string `json:"references"`
	Links      []string `json:"links"`
	Count      int      `json:"count"`
	Bytes      int64    `json:"bytes"`
}

type ontologyManifest struct {
	ProjectID        string            `json:"projectId"`
	SourceType       string            `json:"sourceType"`
	SourceID         string            `json:"sourceId"`
	SourceName       string            `json:"sourceName"`
	InferenceProfile string            `json:"inferenceProfile"`
	DatasetID        string            `json:"datasetId"`
	DatasetName      string            `json:"datasetName"`
	Study            string            `json:"study"`
	Summary          ontologySummary   `json:"summary"`
	Subjects         []ontologySubject `json:"subjects"`
	GeneratedBy      string            `json:"generatedBy"`
	GeneratedAt      time.Time         `json:"generatedAt"`
	Truncated        bool              `json:"truncated"`
}

type ontologyListItem struct {
	ID               string          `json:"id"`
	ProjectID        string          `json:"projectId"`
	ProjectName      string          `json:"projectName"`
	SourceType       string          `json:"sourceType"`
	SourceID         string          `json:"sourceId"`
	SourceName       string          `json:"sourceName"`
	InferenceProfile string          `json:"inferenceProfile"`
	DatasetID        string          `json:"datasetId"`
	DatasetName      string          `json:"datasetName"`
	Study            string          `json:"study"`
	Summary          ontologySummary `json:"summary"`
	GeneratedBy      string          `json:"generatedBy"`
	GeneratedAt      time.Time       `json:"generatedAt"`
	Truncated        bool            `json:"truncated"`
}

type ontologySummary struct {
	Subjects          int      `json:"subjects"`
	Visits            int      `json:"visits"`
	Modalities        int      `json:"modalities"`
	Objects           int      `json:"objects"`
	TotalBytes        int64    `json:"totalBytes"`
	Formats           []string `json:"formats"`
	MeasurementTables []string `json:"measurementTables"`
	// Unrecognised counts the objects whose path matched no subject, and
	// LayoutSamples describes the shape of a few of them. Without these a scan
	// of a dataset laid out differently reported "24,179 objects, 0 subjects"
	// and read as a broken feature rather than as "this layout is not the one
	// the profile knows".
	Unrecognised  int      `json:"unrecognisedObjects"`
	LayoutSamples []string `json:"layoutSamples,omitempty"`
	// RecognisedLayouts describes the shapes that did produce a subject.
	//
	// LayoutSamples above answers "why did the scan read nothing", so it only
	// ever carried the shapes that failed. That is the wrong half for somebody
	// trying to describe a dataset: on SELENA the scan recognised 4,023 objects
	// out of 4,026, and the only shapes recorded were the three it had missed.
	// An assistant shown those learns nothing, and said so - correctly, since
	// it had been told nothing about the data.
	//
	// Shapes, never paths. These are health-context metadata and the shape is
	// what a reading is proposed from; the identifiers are not needed and must
	// not travel.
	RecognisedLayouts []string `json:"recognisedLayouts,omitempty"`
}

type ontologySubject struct {
	ID     string          `json:"id"`
	Visits []ontologyVisit `json:"visits"`
	Stats  ontologySummary `json:"stats"`
}

type ontologyVisit struct {
	Date       string             `json:"date"`
	Modalities []ontologyModality `json:"modalities"`
}

type ontologyModality struct {
	Name              string   `json:"name"`
	ObjectCount       int      `json:"objectCount"`
	TotalBytes        int64    `json:"totalBytes"`
	Formats           []string `json:"formats"`
	MeasurementTables []string `json:"measurementTables"`
	SamplePaths       []string `json:"samplePaths"`
}

type ontologySubjectAcc struct {
	visits map[string]*ontologyVisitAcc
}

type ontologyVisitAcc struct {
	modalities map[string]*ontologyModalityAcc
}

type ontologyModalityAcc struct {
	objectCount       int
	totalBytes        int64
	formats           map[string]struct{}
	measurementTables map[string]struct{}
	samplePaths       []string
}

func (h Handlers) GetProjectOntology(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID is required"})
		return
	}
	if !h.requireProjectMember(w, projectID, userID, "ontology access") {
		return
	}
	raw, found, err := h.projectOntologyStore.GetProjectOntology(projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read project ontology"})
		return
	}
	if !found {
		writeJSON(w, http.StatusOK, map[string]any{"manifest": nil})
		return
	}
	var manifest any
	if err := json.Unmarshal(raw, &manifest); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "stored project ontology is invalid"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"manifest": manifest})
}

func (h Handlers) ListOntologies(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	items, err := h.ontologiesVisibleTo(identity)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list ontologies"})
		return
	}
	h.nameOntologyOwners(items)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) QueryOntology(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.ontologyStore.GetByID(strings.TrimSpace(r.PathValue("ontologyID")))
	if err != nil || !found || (!h.isGlobalAdmin(identity) && h.ontologyRole(item, identity) == "") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}
	var req ontologyQueryRequest
	if r.Body != nil && r.Body != http.NoBody {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid ontology query"})
			return
		}
	}
	var manifest ontologyManifest
	if err := json.Unmarshal(item.Manifest, &manifest); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "stored ontology manifest is invalid"})
		return
	}
	results, total := queryOntologyManifest(manifest, req)
	writeJSON(w, http.StatusOK, map[string]any{
		"items":   results,
		"count":   total,
		"limited": len(results) < total,
	})
}

func (h Handlers) UpdateOntologyMetadata(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.ontologyStore.GetByID(strings.TrimSpace(r.PathValue("ontologyID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}
	if !h.canManageOntologyAccess(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "ontology owner or global admin required"})
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
	if err := h.ontologyStore.UpdateMetadata(item.ID, req.Name, req.Description); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update ontology"})
		return
	}
	updated, _, _ := h.ontologyStore.GetByID(item.ID)
	writeJSON(w, http.StatusOK, updated)
}

func (h Handlers) DeleteOntology(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.ontologyStore.GetByID(strings.TrimSpace(r.PathValue("ontologyID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}
	if !h.canManageOntologyAccess(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "ontology owner or global admin required"})
		return
	}
	if err := h.ontologyStore.Delete(item.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete ontology"})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "ontology.delete", "ontology", item.ID, "", "success", "", map[string]any{"name": item.Name, "sourceType": item.SourceType, "sourceId": item.SourceID})
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) ListOntologyAccess(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.ontologyStore.GetByID(strings.TrimSpace(r.PathValue("ontologyID")))
	if err != nil || !found || (h.ontologyRole(item, identity) == "" && !h.isGlobalAdmin(identity)) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}
	items, err := h.ontologyStore.ListAccess(item.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list ontology permissions"})
		return
	}
	owner := ontologydomain.Access{OntologyID: item.ID, UserID: item.OwnerID, SubjectType: item.OwnerType, SubjectID: item.OwnerID, Role: "owner", CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt}
	writeJSON(w, http.StatusOK, map[string]any{"items": append([]ontologydomain.Access{owner}, items...), "canManage": h.canManageOntologyAccess(item, identity)})
}

func (h Handlers) SetOntologyAccess(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.ontologyStore.GetByID(strings.TrimSpace(r.PathValue("ontologyID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}
	if !h.canManageOntologyAccess(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "ontology owner or global admin required"})
		return
	}
	subjectType := strings.TrimSpace(r.PathValue("subjectType"))
	subjectID := strings.TrimSpace(r.PathValue("subjectID"))
	var req setDatasetAccessRequest
	if !isGrantableSubjectType(subjectType) || subjectID == "" || json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid subjectType, subjectID, and role are required"})
		return
	}
	req.Role = strings.ToLower(strings.TrimSpace(req.Role))
	if req.Role != "reader" && req.Role != "writer" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "role must be reader or writer"})
		return
	}
	if subjectType == "organization" {
		organization, found := h.resolveOrganization(subjectID)
		if !found {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no organization named " + subjectID})
			return
		}
		subjectID = organization.ID
	}
	if subjectType == "team" {
		item, found := h.resolveTeam(subjectID)
		if !found {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no team with identifier " + subjectID})
			return
		}
		subjectID = item.ID
	}
	if strings.EqualFold(subjectType, item.OwnerType) && strings.EqualFold(subjectID, item.OwnerID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "owner role cannot be changed"})
		return
	}
	now := time.Now().UTC()
	access := ontologydomain.Access{OntologyID: item.ID, UserID: subjectID, SubjectType: subjectType, SubjectID: subjectID, Role: req.Role, CreatedAt: now, UpdatedAt: now}
	if existing, exists, _ := h.ontologyStore.GetAccess(item.ID, subjectType, subjectID); exists {
		access.CreatedAt = existing.CreatedAt
	}
	if err := h.ontologyStore.SetAccess(access); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to set ontology permission"})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "ontology.access.set", "ontology", item.ID, "", "success", "", map[string]any{"subjectType": subjectType, "subjectId": subjectID, "role": req.Role})
	writeJSON(w, http.StatusOK, access)
}

func (h Handlers) DeleteOntologyAccess(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.ontologyStore.GetByID(strings.TrimSpace(r.PathValue("ontologyID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}
	if !h.canManageOntologyAccess(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "ontology owner or global admin required"})
		return
	}
	subjectType := strings.TrimSpace(r.PathValue("subjectType"))
	subjectID := strings.TrimSpace(r.PathValue("subjectID"))
	if strings.EqualFold(subjectType, item.OwnerType) && strings.EqualFold(subjectID, item.OwnerID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "owner permission cannot be removed"})
		return
	}
	if err := h.ontologyStore.DeleteAccess(item.ID, subjectType, subjectID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete ontology permission"})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "ontology.access.delete", "ontology", item.ID, "", "success", "", map[string]any{"subjectType": subjectType, "subjectId": subjectID})
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) UpdateOntologyOwner(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.ontologyStore.GetByID(strings.TrimSpace(r.PathValue("ontologyID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}
	if !h.canManageOntologyAccess(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "ontology owner or global admin required"})
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
	if err := h.ontologyStore.UpdateOwner(item.ID, req.OwnerType, req.OwnerID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update ontology owner"})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "ontology.owner.transfer", "ontology", item.ID, "", "success", "", map[string]any{"previousOwnerType": item.OwnerType, "previousOwnerId": item.OwnerID, "ownerType": req.OwnerType, "ownerId": req.OwnerID})
	updated, _, _ := h.ontologyStore.GetByID(item.ID)
	writeJSON(w, http.StatusOK, updated)
}

func (h Handlers) ListProjectOntologies(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	userID := identity.UserID()
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID is required"})
		return
	}
	if !h.requireProjectMember(w, projectID, userID, "ontology listing") {
		return
	}
	ids, err := h.projectResourceStore.ListProjectOntologyIDs(projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list project ontologies"})
		return
	}
	projects, err := h.projectStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list projects"})
		return
	}
	byID := map[string]string{}
	for _, p := range projects {
		byID[p.ID] = p.Name
	}
	items := make([]ontologyListItem, 0, len(ids))
	for _, id := range ids {
		item, found, err := h.ontologyItem(id, byID[projectID])
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load project ontology"})
			return
		}
		if found && h.canReadOntologyObjectID(item.ID, identity) {
			items = append(items, item)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) AttachProjectOntology(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	userID := identity.UserID()
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	ontologyID := strings.TrimSpace(r.PathValue("ontologyID"))
	if projectID == "" || ontologyID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID and ontologyID are required"})
		return
	}
	if !h.requireProjectRole(w, projectID, userID, actionAttachOntology, "ontology attach") {
		return
	}
	item, found, err := h.ontologyStore.GetByID(ontologyID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read ontology"})
		return
	}
	if !found || (h.ontologyRole(item, identity) == "" && !h.isGlobalAdmin(identity)) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "ontology not found"})
		return
	}
	if err := h.projectResourceStore.AttachOntology(projectID, ontologyID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to attach ontology"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) DetachProjectOntology(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	ontologyID := strings.TrimSpace(r.PathValue("ontologyID"))
	if projectID == "" || ontologyID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID and ontologyID are required"})
		return
	}
	if !h.requireProjectRole(w, projectID, userID, actionAttachOntology, "ontology detach") {
		return
	}
	if err := h.projectResourceStore.DetachOntology(projectID, ontologyID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to detach ontology"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ScanOntology builds an ontology from a dataset, with no project involved.
//
// An ontology describes a dataset: its table carries an owner, a source and an
// inference profile, and no project at all. Asking for one to create it was a
// permission anchor wearing the clothes of a parent - you had to pick a
// project before you could photograph a bucket, and the ontology was then
// attached to whichever one you picked.
//
// The project route below still works, and still attaches, because clients and
// screens use it. This one is the shape the object actually has.
func (h Handlers) ScanOntology(w http.ResponseWriter, r *http.Request) {
	h.scanOntology(w, r, "")
}

func (h Handlers) ScanProjectOntology(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID is required"})
		return
	}
	if !h.requireProjectRole(w, projectID, identity.UserID(), actionAttachOntology, "ontology scan") {
		return
	}
	h.scanOntology(w, r, projectID)
}

func (h Handlers) scanOntology(w http.ResponseWriter, r *http.Request, projectID string) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	var req ontologyScanRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	datasetID := strings.TrimSpace(req.DatasetID)
	datasourceID := strings.TrimSpace(req.DatasourceID)
	sourceType := strings.ToLower(strings.TrimSpace(req.SourceType))
	inferenceProfile := strings.TrimSpace(req.InferenceProfile)
	var manifest ontologyManifest
	var scannedObjects []ontologydomain.Object
	if sourceType == "" {
		switch {
		case datasetID != "":
			sourceType = "dataset"
		case datasourceID != "":
			sourceType = "datasource"
		}
	}
	switch sourceType {
	case "dataset", "":
		if inferenceProfile == "" {
			inferenceProfile = "health-file-path-v1"
		}
		if inferenceProfile != "health-file-path-v1" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported dataset inference profile"})
			return
		}
		if datasetID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "datasetId is required"})
			return
		}
		item, found, err := h.datasetStore.GetByID(datasetID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read dataset"})
			return
		}
		if !found || !h.canReadDataset(item, identity) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
			return
		}
		client, _, err := h.datasetS3Client(item)
		if err != nil || client == nil {
			writeJSON(w, http.StatusNotImplemented, map[string]string{"error": datasetS3Error(err)})
			return
		}
		manifest, scannedObjects, err = h.buildDatasetOntologyManifest(r.Context(), projectID, item, client, identity.UserID())
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "ontology scan failed: " + err.Error()})
			return
		}
		// A scan that recognised nothing does not produce an ontology.
		//
		// It used to. Zero subjects and twenty-four thousand unrecognised
		// objects still created a valid, empty ontology, attached it to the
		// project, and answered 201 - so the layout mismatch arrived as an
		// object that looked like it had worked, and the shapes explaining why
		// sat in a field nobody opens. SELENA would have done exactly that
		// this afternoon: its patients are numbered by centre, which the
		// profile did not read as subjects.
		//
		// Refused with the shapes instead, because the shapes are the answer:
		// they say what the layout looks like without saying what it contains,
		// and they are what somebody needs in order to fix the profile or the
		// export.
		if manifest.Summary.Subjects == 0 {
			writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
				"error": fmt.Sprintf(
					"no subject recognised in %d object(s): this dataset's layout is not the one %s reads, so no ontology was created",
					manifest.Summary.Objects+manifest.Summary.Unrecognised, inferenceProfile),
				"code":                "layout_not_recognised",
				"unrecognisedObjects": manifest.Summary.Unrecognised,
				"layoutSamples":       manifest.Summary.LayoutSamples,
				"inferenceProfile":    inferenceProfile,
			})
			return
		}
		manifest.InferenceProfile = inferenceProfile
	case "datasource":
		if inferenceProfile == "" {
			inferenceProfile = "datasource-metadata-v1"
		}
		if inferenceProfile != "datasource-metadata-v1" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported datasource inference profile"})
			return
		}
		if datasourceID == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "datasourceId is required"})
			return
		}
		item, found, err := h.datasourceStore.GetByID(datasourceID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read datasource"})
			return
		}
		if !found || (item.OwnerUserID != identity.UserID() && !h.isGlobalAdmin(identity)) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "datasource not found"})
			return
		}
		manifest = h.buildDatasourceOntologyManifest(projectID, item, identity.UserID())
		manifest.InferenceProfile = inferenceProfile
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "supported ontology source types: dataset, datasource"})
		return
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to encode ontology"})
		return
	}
	objectName := ontologyObjectName(manifest)

	// A second scan of the same source refreshes the ontology it already
	// produced, instead of standing a duplicate beside it.
	//
	// Scanning always inserted, so scanning a dataset twice left two
	// ontologies carrying the same name over the same bucket, distinguishable
	// only by their dates - which is how EMSE ended up with two PREMYOM1000,
	// and the person who scanned had no way to tell which one anybody else was
	// looking at.
	//
	// A re-scan is a new photograph of the same thing. It replaces the picture
	// and keeps the object: the identifier other things point at, the name
	// somebody may have corrected, the owner, the permissions, and the
	// extracts already declared over it - whose file lists are frozen and
	// therefore unaffected by the listing changing underneath.
	//
	// Only among the ontologies the caller can already see: refreshing one
	// that is invisible to them would be editing somebody else's object
	// through a scan.
	object, refreshed, err := h.ontologyToRefresh(identity, manifest)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read ontologies"})
		return
	}
	if refreshed {
		if err := h.ontologyStore.ReplaceManifest(object.ID, raw, identity.UserID()); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to refresh ontology object"})
			return
		}
		object.Manifest = raw
	} else {
		object = ontologydomain.New(identity.UserID(), objectName, "Brouillon genere automatiquement depuis "+manifest.SourceType, manifest.SourceType, manifest.SourceID, manifest.SourceName, manifest.InferenceProfile, raw)
		if err := h.ontologyStore.Create(object); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create ontology object"})
			return
		}
	}
	// Best effort, and said out loud when it fails: an ontology whose paths
	// were not stored still describes the study correctly, it just cannot have
	// an extract built from it - which is a thing to log, not a reason to throw
	// away a scan that took minutes.
	if len(scannedObjects) > 0 {
		if err := h.ontologyStore.ReplaceObjects(object.ID, scannedObjects); err != nil {
			log.Printf("ontology %s stored without its file list; extracts cannot be built from it: %v", object.ID, err)
		}
	}
	// Attached only when a project asked. Scanning from the dataset produces
	// an ontology that belongs to nobody's project until somebody attaches it,
	// which is what the object itself has always said.
	if projectID != "" {
		if err := h.projectResourceStore.AttachOntology(projectID, object.ID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to attach ontology"})
			return
		}
	}
	// 200 when an existing ontology was refreshed, 201 when one was created,
	// and "refreshed" in the body so a screen can say which happened rather
	// than leaving somebody to count rows.
	status := http.StatusCreated
	if refreshed {
		status = http.StatusOK
	}
	writeJSON(w, status, map[string]any{"manifest": manifest, "item": object, "refreshed": refreshed})
}

// ontologyToRefresh finds the ontology a re-scan should replace: same source,
// same inference profile, among those the caller can see. The most recently
// updated one when there are several, which is the one the screens show first.
func (h Handlers) ontologyToRefresh(identity auth.Identity, manifest ontologyManifest) (ontologydomain.Ontology, bool, error) {
	sourceID := strings.TrimSpace(manifest.SourceID)
	if sourceID == "" {
		return ontologydomain.Ontology{}, false, nil
	}
	visibles, err := h.ontologiesVisibleTo(identity)
	if err != nil {
		return ontologydomain.Ontology{}, false, err
	}
	var choisie ontologydomain.Ontology
	trouvee := false
	for _, item := range visibles {
		if strings.TrimSpace(item.SourceID) != sourceID {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(item.SourceType), strings.TrimSpace(manifest.SourceType)) {
			continue
		}
		if !strings.EqualFold(strings.TrimSpace(item.InferenceProfile), strings.TrimSpace(manifest.InferenceProfile)) {
			continue
		}
		if !trouvee || item.UpdatedAt.After(choisie.UpdatedAt) {
			choisie, trouvee = item, true
		}
	}
	return choisie, trouvee, nil
}

func (h Handlers) ontologyItem(ontologyID, projectName string) (ontologyListItem, bool, error) {
	object, found, err := h.ontologyStore.GetByID(ontologyID)
	if err != nil || !found {
		return ontologyListItem{}, found, err
	}
	var manifest ontologyManifest
	if err := json.Unmarshal(object.Manifest, &manifest); err != nil {
		return ontologyListItem{}, false, err
	}
	if manifest.ProjectID == "" {
		manifest.ProjectID = object.ID
	}
	if manifest.SourceType == "" {
		manifest.SourceType = object.SourceType
	}
	if manifest.SourceID == "" {
		manifest.SourceID = firstNonEmpty(object.SourceID, manifest.DatasetID)
	}
	if manifest.SourceName == "" {
		manifest.SourceName = firstNonEmpty(object.SourceName, manifest.DatasetName)
	}
	if manifest.InferenceProfile == "" {
		if object.InferenceProfile != "" {
			manifest.InferenceProfile = object.InferenceProfile
		} else if manifest.SourceType == "datasource" {
			manifest.InferenceProfile = "datasource-metadata-v1"
		} else {
			manifest.InferenceProfile = "health-file-path-v1"
		}
	}
	return ontologyListItem{
		ID:               object.ID,
		ProjectID:        manifest.ProjectID,
		ProjectName:      projectName,
		SourceType:       manifest.SourceType,
		SourceID:         manifest.SourceID,
		SourceName:       manifest.SourceName,
		InferenceProfile: manifest.InferenceProfile,
		DatasetID:        manifest.DatasetID,
		DatasetName:      manifest.DatasetName,
		Study:            manifest.Study,
		Summary:          manifest.Summary,
		GeneratedBy:      manifest.GeneratedBy,
		GeneratedAt:      manifest.GeneratedAt,
		Truncated:        manifest.Truncated,
	}, true, nil
}

func (h Handlers) ontologySubjects(identity auth.Identity) []ontologydomain.Subject {
	subjects := []ontologydomain.Subject{{Type: "user", ID: identity.UserID()}}
	if h.keycloak == nil {
		return subjects
	}
	identifier := strings.TrimSpace(identity.Subject)
	if identifier == "" {
		identifier = identity.UserID()
	}
	organizations, err := h.keycloak.ListUserOrganizations(identifier)
	if err != nil {
		return subjects
	}
	for _, organization := range organizations {
		subjects = append(subjects, ontologydomain.Subject{Type: "organization", ID: organization.ID})
	}
	// Les equipes, que les datasets connaissaient deja et pas les ontologies :
	// une ontologie ne pouvait donc meme pas etre partagee avec une equipe.
	for _, teamID := range h.callerTeamIDs(identity) {
		subjects = append(subjects, ontologydomain.Subject{Type: "team", ID: teamID})
	}
	return subjects
}

func (h Handlers) ontologyRole(item ontologydomain.Ontology, identity auth.Identity) string {
	best := ""
	for _, subject := range h.ontologySubjects(identity) {
		if strings.EqualFold(item.OwnerType, subject.Type) && strings.EqualFold(item.OwnerID, subject.ID) {
			return "owner"
		}
		access, found, err := h.ontologyStore.GetAccess(item.ID, subject.Type, subject.ID)
		if err == nil && found && (access.Role == "writer" || best == "") {
			best = access.Role
		}
	}
	return best
}

func (h Handlers) canReadOntologyObjectID(ontologyID string, identity auth.Identity) bool {
	item, found, err := h.ontologyStore.GetByID(ontologyID)
	if err != nil || !found {
		return false
	}
	return h.isGlobalAdmin(identity) || h.ontologyRole(item, identity) != ""
}

func (h Handlers) canManageOntologyAccess(item ontologydomain.Ontology, identity auth.Identity) bool {
	return h.isGlobalAdmin(identity) || h.ontologyRole(item, identity) == "owner"
}

func ontologyObjectName(manifest ontologyManifest) string {
	if strings.TrimSpace(manifest.Study) != "" {
		return strings.TrimSpace(manifest.Study)
	}
	if strings.TrimSpace(manifest.SourceName) != "" {
		return strings.TrimSpace(manifest.SourceName)
	}
	if strings.TrimSpace(manifest.DatasetName) != "" {
		return strings.TrimSpace(manifest.DatasetName)
	}
	return "Catalogue semantique"
}

func queryOntologyManifest(manifest ontologyManifest, req ontologyQueryRequest) ([]ontologyQueryItem, int) {
	limit := queryLimit(req.Limit)
	items := make([]ontologyQueryItem, 0)
	total := 0
	for _, subject := range manifest.Subjects {
		for _, visit := range subject.Visits {
			for _, modality := range visit.Modalities {
				item := ontologyManifestQueryItem(manifest, subject, visit, modality)
				if !ontologyQueryMatches(item, req) {
					continue
				}
				total++
				if len(items) < limit {
					items = append(items, item)
				}
			}
		}
	}
	return items, total
}

func ontologyManifestQueryItem(manifest ontologyManifest, subject ontologySubject, visit ontologyVisit, modality ontologyModality) ontologyQueryItem {
	sourceType := strings.TrimSpace(manifest.SourceType)
	object := strings.TrimSpace(subject.ID)
	parent := strings.TrimSpace(visit.Date)
	typ := strings.TrimSpace(modality.Name)
	if sourceType == "datasource" {
		object = firstNonEmpty(manifest.SourceName, manifest.SourceID, modality.Name)
		parent = firstNonEmpty(manifest.SourceName, manifest.SourceID, subject.ID)
	}
	return ontologyQueryItem{
		Object:     firstNonEmpty(object, "-"),
		Type:       firstNonEmpty(typ, "object"),
		Parent:     firstNonEmpty(parent, "-"),
		Attributes: compactStrings(modality.Formats),
		References: compactStrings(modality.MeasurementTables),
		Links:      compactStrings(modality.SamplePaths),
		Count:      modality.ObjectCount,
		Bytes:      modality.TotalBytes,
	}
}

func ontologyQueryMatches(item ontologyQueryItem, req ontologyQueryRequest) bool {
	if !queryContains(item.Object, req.Object) && !queryContains(strings.Join([]string{item.Object, item.Type, item.Parent}, " "), req.Object) {
		return false
	}
	if !queryContains(item.Type, req.Type) {
		return false
	}
	if !queryContains(item.Parent, req.Parent) {
		return false
	}
	if !querySliceContains(item.Attributes, req.Attribute) {
		return false
	}
	if !querySliceContains(item.References, req.Reference) {
		return false
	}
	return true
}

func queryContains(value, needle string) bool {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return true
	}
	return strings.Contains(strings.ToLower(strings.TrimSpace(value)), needle)
}

func querySliceContains(values []string, needle string) bool {
	needle = strings.ToLower(strings.TrimSpace(needle))
	if needle == "" {
		return true
	}
	for _, value := range values {
		if strings.Contains(strings.ToLower(strings.TrimSpace(value)), needle) {
			return true
		}
	}
	return false
}

func queryLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

// The scan returns the file list alongside the manifest. The manifest keeps
// three sample paths per modality - enough to show what the data looks like,
// never enough to build an extract from - so the recognised paths are handed back
// to be stored, and an extract declared next month can still name the same files.
func (h Handlers) buildDatasetOntologyManifest(ctx context.Context, projectID string, item dataset.Dataset, client *minio.Client, generatedBy string) (ontologyManifest, []ontologydomain.Object, error) {
	prefix := strings.Trim(item.Prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	scanCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	subjects := map[string]*ontologySubjectAcc{}
	formats := map[string]struct{}{}
	tables := map[string]struct{}{}
	modalities := map[string]struct{}{}
	study := ""
	objects := 0
	unrecognised := 0
	layouts := map[string]int{}
	// The shapes that worked, kept apart from the ones that did not: one
	// explains a refusal, the other describes the data.
	reconnus := map[string]int{}
	var totalBytes int64
	truncated := false

	recognised := []ontologydomain.Object{}
	for obj := range client.ListObjects(scanCtx, item.Bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return ontologyManifest{}, nil, obj.Err
		}
		relPath := obj.Key
		if prefix != "" && strings.HasPrefix(relPath, prefix) {
			relPath = strings.TrimPrefix(relPath, prefix)
		}
		relPath = strings.Trim(relPath, "/")
		if relPath == "" {
			continue
		}
		objects++
		totalBytes += obj.Size
		if objects > ontologyScanMaxObjects {
			truncated = true
			break
		}
		subjectID, visitDate, modalityName := inferOntologyPath(relPath)
		if subjectID == "" {
			// Counted and described, never dropped in silence. The shape is
			// recorded rather than the path: these are health-context
			// metadata, and a diagnosis does not need the identifiers.
			unrecognised++
			// Every shape is counted, and the frequent ones are what get
			// shown. Keeping only the first few *encountered* made the sample
			// unrepresentative: on HDS-For it reported six shapes totalling 14
			// objects out of 2,550, which says nothing about the 2,536 others.
			// The cap is on distinct shapes, which is bounded in practice.
			if shape := describePathShape(relPath); len(layouts) < 64 || layouts[shape] > 0 {
				layouts[shape]++
			}
			continue
		}
		if shape := describePathShape(relPath); len(reconnus) < 64 || reconnus[shape] > 0 {
			reconnus[shape]++
		}
		if study == "" {
			study = inferStudy(subjectID)
		}
		if visitDate == "" {
			visitDate = "unknown"
		}
		if modalityName == "" {
			modalityName = "unknown"
		}
		format := inferObjectFormat(relPath)
		if format != "" {
			formats[format] = struct{}{}
		}
		table := inferMeasurementTable(relPath, format)
		if table != "" {
			tables[table] = struct{}{}
		}
		modalities[modalityName] = struct{}{}
		acc := getOntologyModalityAcc(subjects, subjectID, visitDate, modalityName)
		acc.objectCount++
		acc.totalBytes += obj.Size
		if format != "" {
			acc.formats[format] = struct{}{}
		}
		if table != "" {
			acc.measurementTables[table] = struct{}{}
		}
		if len(acc.samplePaths) < 3 {
			acc.samplePaths = append(acc.samplePaths, relPath)
		}
		recognised = append(recognised, ontologydomain.Object{
			Path:      relPath,
			SubjectID: subjectID,
			Visit:     visitDate,
			Modality:  modalityName,
			SizeBytes: obj.Size,
		})
	}
	if study == "" {
		study = strings.TrimSpace(item.Name)
	}
	manifestSubjects, visitCount := materializeOntologySubjects(subjects)
	return ontologyManifest{
		ProjectID:        projectID,
		SourceType:       "dataset",
		SourceID:         item.ID,
		SourceName:       item.Name,
		InferenceProfile: "health-file-path-v1",
		DatasetID:        item.ID,
		DatasetName:      item.Name,
		Study:            study,
		Summary: ontologySummary{
			Subjects:          len(manifestSubjects),
			Visits:            visitCount,
			Modalities:        len(modalities),
			Objects:           objects,
			TotalBytes:        totalBytes,
			Formats:           sortedKeys(formats),
			MeasurementTables: sortedKeys(tables),
			Unrecognised:      unrecognised,
			LayoutSamples:     describeLayouts(layouts),
			RecognisedLayouts: describeLayouts(reconnus),
		},
		Subjects:    manifestSubjects,
		GeneratedBy: generatedBy,
		GeneratedAt: time.Now().UTC(),
		Truncated:   truncated,
	}, recognised, nil
}

func (h Handlers) buildDatasourceOntologyManifest(projectID string, item datasource.Datasource, generatedBy string) ontologyManifest {
	source := strings.TrimSpace(item.Source)
	if source == "" {
		source = "external"
	}
	labels := []string{strings.ToLower(strings.TrimSpace(item.Type)), source}
	if item.ServiceDefinitionID != "" {
		labels = append(labels, item.ServiceDefinitionID)
	}
	tables := []string{}
	if item.Database != "" {
		tables = append(tables, item.Database)
	}
	modalityName := strings.ToUpper(strings.TrimSpace(item.Type))
	if modalityName == "" {
		modalityName = "DATASOURCE"
	}
	host := item.Host
	if item.Port > 0 {
		host = host + ":" + strconv.Itoa(item.Port)
	}
	return ontologyManifest{
		ProjectID:        strings.TrimSpace(projectID),
		SourceType:       "datasource",
		SourceID:         item.ID,
		SourceName:       item.Name,
		InferenceProfile: "datasource-metadata-v1",
		Study:            item.Name,
		GeneratedBy:      strings.TrimSpace(generatedBy),
		GeneratedAt:      time.Now().UTC(),
		Summary: ontologySummary{
			Subjects:          1,
			Visits:            1,
			Modalities:        1,
			Objects:           1,
			Formats:           compactStrings(labels),
			MeasurementTables: compactStrings(tables),
		},
		Subjects: []ontologySubject{{
			ID: "datasource",
			Visits: []ontologyVisit{{
				Date: "live",
				Modalities: []ontologyModality{{
					Name:              modalityName,
					ObjectCount:       1,
					Formats:           compactStrings(labels),
					MeasurementTables: compactStrings(tables),
					SamplePaths:       compactStrings([]string{host, item.Database, item.ServiceName}),
				}},
			}},
			Stats: ontologySummary{Objects: 1, Modalities: 1, Visits: 1, Formats: compactStrings(labels), MeasurementTables: compactStrings(tables)},
		}},
	}
}

func inferOntologyPath(relPath string) (subjectID, visitDate, modality string) {
	parts := strings.Split(strings.Trim(relPath, "/"), "/")
	for i, part := range parts {
		if !ontologySubjectPattern.MatchString(part) {
			continue
		}
		subjectID = part
		if i+1 < len(parts) {
			candidate := strings.TrimPrefix(parts[i+1], "visit_")
			if ontologyDatePattern.MatchString(candidate) {
				visitDate = candidate
			}
		}
		if i+2 < len(parts) {
			modality = strings.TrimPrefix(parts[i+2], "modality_")
			modality = strings.ToUpper(strings.TrimSpace(modality))
		}
		return subjectID, visitDate, modality
	}
	return "", "", ""
}

// describePathShape says what a path looks like without saying what it says:
// "8 levels: word/word/id-0000/date/word/word/word/file.dcm" becomes
// "8 levels · text/text/subject/date/text/text/text/DICOM". A layout that the
// profile does not understand can then be read at a glance, and nothing
// identifying leaves the scan.
func describePathShape(relPath string) string {
	parts := strings.Split(strings.Trim(relPath, "/"), "/")
	shapes := make([]string, 0, len(parts))
	for index, part := range parts {
		switch {
		case index == len(parts)-1 && strings.Contains(part, "."):
			if format := inferObjectFormat(part); format != "" {
				shapes = append(shapes, format)
			} else {
				shapes = append(shapes, "file")
			}
		case ontologySubjectPattern.MatchString(part):
			shapes = append(shapes, "subject")
		case ontologyDatePattern.MatchString(strings.TrimPrefix(part, "visit_")):
			shapes = append(shapes, "date")
		case isAllDigits(part):
			shapes = append(shapes, "number")
		default:
			shapes = append(shapes, "text")
		}
	}
	return fmt.Sprintf("%d levels · %s", len(parts), strings.Join(shapes, "/"))
}

func isAllDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// describeLayouts returns the most common shapes first: the point is to show
// somebody the convention their data actually follows.
func describeLayouts(layouts map[string]int) []string {
	if len(layouts) == 0 {
		return nil
	}
	type entry struct {
		shape string
		count int
	}
	entries := make([]entry, 0, len(layouts))
	for shape, count := range layouts {
		entries = append(entries, entry{shape: shape, count: count})
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].shape < entries[j].shape
	})
	if len(entries) > 8 {
		entries = entries[:8]
	}
	out := make([]string, 0, len(entries))
	for _, item := range entries {
		out = append(out, fmt.Sprintf("%s (%d)", item.shape, item.count))
	}
	return out
}

func inferStudy(subjectID string) string {
	idx := strings.LastIndex(subjectID, "-")
	if idx > 0 {
		return subjectID[:idx]
	}
	return subjectID
}

func inferObjectFormat(relPath string) string {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(relPath), "."))
	switch ext {
	case "dcm":
		return "DICOM"
	case "csv":
		return "CSV"
	case "tsv":
		return "TSV"
	case "xml":
		return "XML"
	case "e2e":
		return "E2E"
	case "png":
		return "PNG"
	case "ib":
		return "IB"
	case "pdf":
		return "PDF"
	case "xlsx", "xls", "ods":
		return strings.ToUpper(ext)
	}
	upper := strings.ToUpper(relPath)
	if strings.Contains(upper, "/DICOM/") || strings.Contains(upper, "DICOMDIR") {
		return "DICOM"
	}
	if ext != "" {
		return strings.ToUpper(ext)
	}
	return "NO_EXT"
}

func inferMeasurementTable(relPath, format string) string {
	if format != "CSV" && format != "TSV" {
		return ""
	}
	base := path.Base(relPath)
	ext := path.Ext(base)
	name := strings.TrimSpace(strings.TrimSuffix(base, ext))
	if ontologySubjectPattern.MatchString(name) || strings.HasPrefix(strings.ToLower(name), "patient_") {
		return ""
	}
	return name
}

func getOntologyModalityAcc(subjects map[string]*ontologySubjectAcc, subjectID, visitDate, modality string) *ontologyModalityAcc {
	subj := subjects[subjectID]
	if subj == nil {
		subj = &ontologySubjectAcc{visits: map[string]*ontologyVisitAcc{}}
		subjects[subjectID] = subj
	}
	visit := subj.visits[visitDate]
	if visit == nil {
		visit = &ontologyVisitAcc{modalities: map[string]*ontologyModalityAcc{}}
		subj.visits[visitDate] = visit
	}
	mod := visit.modalities[modality]
	if mod == nil {
		mod = &ontologyModalityAcc{formats: map[string]struct{}{}, measurementTables: map[string]struct{}{}}
		visit.modalities[modality] = mod
	}
	return mod
}

func materializeOntologySubjects(subjects map[string]*ontologySubjectAcc) ([]ontologySubject, int) {
	ids := make([]string, 0, len(subjects))
	for id := range subjects {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]ontologySubject, 0, len(ids))
	visitCount := 0
	for _, id := range ids {
		acc := subjects[id]
		visitDates := make([]string, 0, len(acc.visits))
		for date := range acc.visits {
			visitDates = append(visitDates, date)
		}
		sort.Strings(visitDates)
		subj := ontologySubject{ID: id, Visits: []ontologyVisit{}}
		subjFormats := map[string]struct{}{}
		subjTables := map[string]struct{}{}
		subjModalities := map[string]struct{}{}
		for _, date := range visitDates {
			visitCount++
			visit := acc.visits[date]
			modNames := make([]string, 0, len(visit.modalities))
			for name := range visit.modalities {
				modNames = append(modNames, name)
			}
			sort.Strings(modNames)
			mods := make([]ontologyModality, 0, len(modNames))
			for _, name := range modNames {
				modAcc := visit.modalities[name]
				for key := range modAcc.formats {
					subjFormats[key] = struct{}{}
				}
				for key := range modAcc.measurementTables {
					subjTables[key] = struct{}{}
				}
				subjModalities[name] = struct{}{}
				subj.Stats.Objects += modAcc.objectCount
				subj.Stats.TotalBytes += modAcc.totalBytes
				mods = append(mods, ontologyModality{
					Name:              name,
					ObjectCount:       modAcc.objectCount,
					TotalBytes:        modAcc.totalBytes,
					Formats:           sortedKeys(modAcc.formats),
					MeasurementTables: sortedKeys(modAcc.measurementTables),
					SamplePaths:       append([]string(nil), modAcc.samplePaths...),
				})
			}
			subj.Visits = append(subj.Visits, ontologyVisit{Date: date, Modalities: mods})
		}
		subj.Stats.Subjects = 1
		subj.Stats.Visits = len(subj.Visits)
		subj.Stats.Modalities = len(subjModalities)
		subj.Stats.Formats = sortedKeys(subjFormats)
		subj.Stats.MeasurementTables = sortedKeys(subjTables)
		out = append(out, subj)
	}
	return out, visitCount
}

func sortedKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		if strings.TrimSpace(value) != "" {
			out = append(out, value)
		}
	}
	sort.Strings(out)
	return out
}

func compactStrings(values []string) []string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	sort.Strings(out)
	return out
}

func stringInSlice(value string, values []string) bool {
	for _, candidate := range values {
		if strings.TrimSpace(candidate) == strings.TrimSpace(value) {
			return true
		}
	}
	return false
}
