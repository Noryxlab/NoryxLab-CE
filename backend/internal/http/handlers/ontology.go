package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/datasource"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	"github.com/minio/minio-go/v7"
)

const ontologyScanMaxObjects = 50000

var (
	ontologySubjectPattern = regexp.MustCompile(`^[A-Za-z0-9]+-[0-9]{3,4}$`)
	ontologyDatePattern    = regexp.MustCompile(`^[0-9]{8}$`)
)

type ontologyScanRequest struct {
	DatasetID    string `json:"datasetId"`
	DatasourceID string `json:"datasourceId"`
	SourceType   string `json:"sourceType"`
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
	items, err := h.ontologyStore.ListBySubjects(h.ontologySubjects(identity))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list ontologies"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
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
	if (subjectType != "user" && subjectType != "organization") || subjectID == "" || json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid subjectType, subjectID, and role are required"})
		return
	}
	req.Role = strings.ToLower(strings.TrimSpace(req.Role))
	if req.Role != "reader" && req.Role != "writer" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "role must be reader or writer"})
		return
	}
	if subjectType == "organization" && !h.organizationExists(subjectID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization does not exist"})
		return
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
	req.OwnerType = strings.ToLower(strings.TrimSpace(req.OwnerType))
	req.OwnerID = strings.TrimSpace(req.OwnerID)
	if (req.OwnerType != "user" && req.OwnerType != "organization") || req.OwnerID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ownerType must be user or organization and ownerId is required"})
		return
	}
	if req.OwnerType == "organization" && !h.organizationExists(req.OwnerID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "organization does not exist"})
		return
	}
	if req.OwnerType == "organization" && !h.isGlobalAdmin(identity) {
		isMember := false
		for _, subject := range h.ontologySubjects(identity) {
			if subject.Type == "organization" && subject.ID == req.OwnerID {
				isMember = true
				break
			}
		}
		if !isMember {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "destination organization membership or global admin required"})
			return
		}
	}
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
	if !h.requireProjectRole(w, projectID, userID, access.Role.CanLaunchPod, "ontology attach") {
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
	if !h.requireProjectRole(w, projectID, userID, access.Role.CanLaunchPod, "ontology detach") {
		return
	}
	if err := h.projectResourceStore.DetachOntology(projectID, ontologyID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to detach ontology"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
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
	if !h.requireProjectRole(w, projectID, identity.UserID(), access.Role.CanLaunchPod, "ontology scan") {
		return
	}
	var req ontologyScanRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	datasetID := strings.TrimSpace(req.DatasetID)
	datasourceID := strings.TrimSpace(req.DatasourceID)
	sourceType := strings.ToLower(strings.TrimSpace(req.SourceType))
	var manifest ontologyManifest
	var sourceID string
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
		manifest, err = h.buildDatasetOntologyManifest(r.Context(), projectID, item, client, identity.UserID())
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "ontology scan failed: " + err.Error()})
			return
		}
		sourceID = item.ID
	case "datasource":
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
		sourceID = item.ID
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
	object := ontologydomain.New(identity.UserID(), objectName, "Brouillon genere automatiquement depuis "+manifest.SourceType, manifest.SourceType, manifest.SourceID, manifest.SourceName, manifest.InferenceProfile, raw)
	if err := h.ontologyStore.Create(object); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create ontology object"})
		return
	}
	if err := h.projectOntologyStore.UpsertProjectOntology(projectID, sourceID, raw, identity.UserID()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store project ontology"})
		return
	}
	if err := h.projectResourceStore.AttachOntology(projectID, object.ID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to attach ontology"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"manifest": manifest, "item": object})
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
			manifest.InferenceProfile = "premyom-file-path-v1"
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

func (h Handlers) buildDatasetOntologyManifest(ctx context.Context, projectID string, item dataset.Dataset, client *minio.Client, generatedBy string) (ontologyManifest, error) {
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
	var totalBytes int64
	truncated := false

	for obj := range client.ListObjects(scanCtx, item.Bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return ontologyManifest{}, obj.Err
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
			continue
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
		InferenceProfile: "premyom-file-path-v1",
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
		},
		Subjects:    manifestSubjects,
		GeneratedBy: generatedBy,
		GeneratedAt: time.Now().UTC(),
		Truncated:   truncated,
	}, nil
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
