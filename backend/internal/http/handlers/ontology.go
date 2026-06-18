package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"path"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/minio/minio-go/v7"
)

const ontologyScanMaxObjects = 50000

var (
	ontologySubjectPattern = regexp.MustCompile(`^[A-Za-z0-9]+-[0-9]{3,4}$`)
	ontologyDatePattern    = regexp.MustCompile(`^[0-9]{8}$`)
)

type ontologyScanRequest struct {
	DatasetID string `json:"datasetId"`
}

type ontologyManifest struct {
	ProjectID   string            `json:"projectId"`
	DatasetID   string            `json:"datasetId"`
	DatasetName string            `json:"datasetName"`
	Study       string            `json:"study"`
	Summary     ontologySummary   `json:"summary"`
	Subjects    []ontologySubject `json:"subjects"`
	GeneratedBy string            `json:"generatedBy"`
	GeneratedAt time.Time         `json:"generatedAt"`
	Truncated   bool              `json:"truncated"`
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
	attachedIDs, err := h.projectResourceStore.ListProjectDatasetIDs(projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list project datasets"})
		return
	}
	datasetID := strings.TrimSpace(req.DatasetID)
	if datasetID == "" && len(attachedIDs) > 0 {
		datasetID = attachedIDs[0]
	}
	if datasetID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "attach a dataset before scanning ontology"})
		return
	}
	if !stringInSlice(datasetID, attachedIDs) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "dataset is not attached to this project"})
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
	manifest, err := h.buildOntologyManifest(r.Context(), projectID, item, client, identity.UserID())
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "ontology scan failed: " + err.Error()})
		return
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to encode ontology"})
		return
	}
	if err := h.projectOntologyStore.UpsertProjectOntology(projectID, datasetID, raw, identity.UserID()); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store project ontology"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"manifest": manifest})
}

func (h Handlers) buildOntologyManifest(ctx context.Context, projectID string, item dataset.Dataset, client *minio.Client, generatedBy string) (ontologyManifest, error) {
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
		ProjectID:   projectID,
		DatasetID:   item.ID,
		DatasetName: item.Name,
		Study:       study,
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

func stringInSlice(value string, values []string) bool {
	for _, candidate := range values {
		if strings.TrimSpace(candidate) == strings.TrimSpace(value) {
			return true
		}
	}
	return false
}
