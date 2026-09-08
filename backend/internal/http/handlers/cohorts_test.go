package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	cohortdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/cohort"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

func cohortFixture(t *testing.T) (Handlers, string) {
	t.Helper()
	ontologies := memory.NewOntologyObjectStore()
	object := ontologydomain.New("user-1", "PREMYOM1000", "", "dataset", "dataset-1", "HDS-For", "health-file-path-v1", []byte(`{}`))
	if err := ontologies.Create(object); err != nil {
		t.Fatalf("create ontology: %v", err)
	}
	if err := ontologies.ReplaceObjects(object.ID, []ontologydomain.Object{
		{Path: "S1/v1/Cornea_Wavefront/a.e2e", SubjectID: "S1", Visit: "v1", Modality: "Cornea_Wavefront", SizeBytes: 10},
		{Path: "S1/v1/Cornea_Basics/b.csv", SubjectID: "S1", Visit: "v1", Modality: "Cornea_Basics", SizeBytes: 20},
		{Path: "S2/v1/Cornea_Basics/c.csv", SubjectID: "S2", Visit: "v1", Modality: "Cornea_Basics", SizeBytes: 30},
	}); err != nil {
		t.Fatalf("store objects: %v", err)
	}
	return Handlers{authMode: "header", ontologyStore: ontologies, cohortStore: memory.NewCohortStore()}, object.ID
}

func postCohort(t *testing.T, h Handlers, ontologyID string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/ontologies/"+ontologyID+"/cohorts", bytes.NewReader(raw))
	request.Header.Set(userHeader, "user-1")
	request.SetPathValue("ontologyID", ontologyID)
	recorder := httptest.NewRecorder()
	h.CreateCohort(recorder, request)
	return recorder
}

// A cohort is frozen when it is declared: the filter selects, and what is kept
// is the list of files, so the same cohort still names the same study after the
// source grows.
func TestCohortFreezesTheFilesItSelected(t *testing.T) {
	h, ontologyID := cohortFixture(t)

	recorder := postCohort(t, h, ontologyID, map[string]any{
		"name":       "Basics",
		"projectId":  "project-1",
		"modalities": []string{"Cornea_Basics"},
	})
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var created cohortdomain.Cohort
	if err := json.Unmarshal(recorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.ObjectCount != 2 || created.TotalBytes != 50 {
		t.Fatalf("cohort froze %d files / %d bytes, want 2 / 50", created.ObjectCount, created.TotalBytes)
	}

	// The source grows; the cohort does not.
	ontologies := h.ontologyStore
	if err := ontologies.ReplaceObjects(ontologyID, []ontologydomain.Object{
		{Path: "S3/v1/Cornea_Basics/d.csv", SubjectID: "S3", Visit: "v1", Modality: "Cornea_Basics", SizeBytes: 40},
	}); err != nil {
		t.Fatalf("rescan: %v", err)
	}
	members, err := h.cohortStore.ListMembers(created.ID, 0)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("cohort now holds %d files; a frozen cohort must not follow its source", len(members))
	}
}

// An empty axis means "no constraint", never "nothing": a cohort named by
// modality alone spans every subject who carries it.
func TestCohortEmptyAxisMeansNoConstraint(t *testing.T) {
	h, ontologyID := cohortFixture(t)

	recorder := postCohort(t, h, ontologyID, map[string]any{"name": "Everything", "projectId": "project-1"})
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var created cohortdomain.Cohort
	_ = json.Unmarshal(recorder.Body.Bytes(), &created)
	if created.ObjectCount != 3 {
		t.Fatalf("cohort froze %d files, want all 3", created.ObjectCount)
	}
}

// An ontology scanned before paths were kept resolves to nothing, and says so:
// a cohort of zero files would look like a legitimate empty result.
func TestCohortRefusesAnOntologyWithoutStoredPaths(t *testing.T) {
	ontologies := memory.NewOntologyObjectStore()
	object := ontologydomain.New("user-1", "Old", "", "dataset", "dataset-1", "HDS-For", "health-file-path-v1", []byte(`{}`))
	if err := ontologies.Create(object); err != nil {
		t.Fatalf("create ontology: %v", err)
	}
	h := Handlers{authMode: "header", ontologyStore: ontologies, cohortStore: memory.NewCohortStore()}

	recorder := postCohort(t, h, object.ID, map[string]any{"name": "Anything", "projectId": "project-1"})
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 with an explanation; body = %s", recorder.Code, recorder.Body.String())
	}
}
