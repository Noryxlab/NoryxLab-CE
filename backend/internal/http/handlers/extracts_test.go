package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

func extractFixture(t *testing.T) (Handlers, string) {
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
	return Handlers{authMode: "header", ontologyStore: ontologies, extractStore: memory.NewExtractStore()}, object.ID
}

func postExtract(t *testing.T, h Handlers, ontologyID string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/ontologies/"+ontologyID+"/extracts", bytes.NewReader(raw))
	request.Header.Set(userHeader, "user-1")
	request.SetPathValue("ontologyID", ontologyID)
	recorder := httptest.NewRecorder()
	h.CreateExtract(recorder, request)
	return recorder
}

// An extract is frozen when it is declared: the filter selects, and what is kept
// is the list of files, so the same extract still names the same study after the
// source grows.
func TestExtractFreezesTheFilesItSelected(t *testing.T) {
	h, ontologyID := extractFixture(t)

	recorder := postExtract(t, h, ontologyID, map[string]any{
		"name":       "Basics",
		"projectId":  "project-1",
		"modalities": []string{"Cornea_Basics"},
	})
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var created extractdomain.Extract
	if err := json.Unmarshal(recorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.ObjectCount != 2 || created.TotalBytes != 50 {
		t.Fatalf("extract froze %d files / %d bytes, want 2 / 50", created.ObjectCount, created.TotalBytes)
	}

	// The source grows; the extract does not.
	ontologies := h.ontologyStore
	if err := ontologies.ReplaceObjects(ontologyID, []ontologydomain.Object{
		{Path: "S3/v1/Cornea_Basics/d.csv", SubjectID: "S3", Visit: "v1", Modality: "Cornea_Basics", SizeBytes: 40},
	}); err != nil {
		t.Fatalf("rescan: %v", err)
	}
	members, err := h.extractStore.ListMembers(created.ID, 0)
	if err != nil {
		t.Fatalf("list members: %v", err)
	}
	if len(members) != 2 {
		t.Fatalf("extract now holds %d files; a frozen extract must not follow its source", len(members))
	}
}

// An empty axis means "no constraint", never "nothing": an extract named by
// modality alone spans every subject who carries it.
func TestExtractEmptyAxisMeansNoConstraint(t *testing.T) {
	h, ontologyID := extractFixture(t)

	recorder := postExtract(t, h, ontologyID, map[string]any{"name": "Everything", "projectId": "project-1"})
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var created extractdomain.Extract
	_ = json.Unmarshal(recorder.Body.Bytes(), &created)
	if created.ObjectCount != 3 {
		t.Fatalf("extract froze %d files, want all 3", created.ObjectCount)
	}
}

// An ontology scanned before paths were kept resolves to nothing, and says so:
// an extract of zero files would look like a legitimate empty result.
func TestExtractRefusesAnOntologyWithoutStoredPaths(t *testing.T) {
	ontologies := memory.NewOntologyObjectStore()
	object := ontologydomain.New("user-1", "Old", "", "dataset", "dataset-1", "HDS-For", "health-file-path-v1", []byte(`{}`))
	if err := ontologies.Create(object); err != nil {
		t.Fatalf("create ontology: %v", err)
	}
	h := Handlers{authMode: "header", ontologyStore: ontologies, extractStore: memory.NewExtractStore()}

	recorder := postExtract(t, h, object.ID, map[string]any{"name": "Anything", "projectId": "project-1"})
	if recorder.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409 with an explanation; body = %s", recorder.Code, recorder.Body.String())
	}
}

// The catalogue lists extracts without naming an ontology, so it needs a rule
// for what a caller may see - and the rule is the ontology's, not the
// extract's.
//
// An extract is a frozen selection over one ontology's subjects: its name, its
// n and its modalities all describe that ontology's content. Listing one drawn
// from an ontology the caller cannot read would publish that description, so
// visibility follows the ontology and ownership only decides who may transfer
// or delete.
func TestTheExtractCatalogueFollowsOntologyVisibility(t *testing.T) {
	ontologies := memory.NewOntologyObjectStore()
	sienne := ontologydomain.New("user-1", "PREMYOM1000", "", "dataset", "dataset-1", "HDS-For", "health-file-path-v1", []byte(`{}`))
	autre := ontologydomain.New("user-2", "Selena", "", "dataset", "dataset-2", "HDS-For", "health-file-path-v1", []byte(`{}`))
	for _, item := range []ontologydomain.Ontology{sienne, autre} {
		if err := ontologies.Create(item); err != nil {
			t.Fatalf("create ontology: %v", err)
		}
	}
	extraits := memory.NewExtractStore()
	for _, item := range []extractdomain.Extract{
		{ID: "e-sienne", OntologyID: sienne.ID, Name: "Basics", OwnerUserID: "user-1", OwnerType: ownerUser, OwnerID: "user-1"},
		{ID: "e-autre", OntologyID: autre.ID, Name: "Selena cohort", OwnerUserID: "user-2", OwnerType: ownerUser, OwnerID: "user-2"},
	} {
		if err := extraits.Create(item, nil); err != nil {
			t.Fatalf("create extract: %v", err)
		}
	}
	h := Handlers{authMode: "header", ontologyStore: ontologies, extractStore: extraits}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/extracts", nil)
	request.Header.Set(userHeader, "user-1")
	recorder := httptest.NewRecorder()
	h.ListExtracts(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var payload struct{ Items []extractdomain.Extract }
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Items) != 1 || payload.Items[0].ID != "e-sienne" {
		t.Fatalf("listed %+v, want only the extract over a readable ontology", payload.Items)
	}
	// Le proprietaire se lit, sinon la colonne du catalogue affiche un UUID.
	if payload.Items[0].OwnerName != "user-1" {
		t.Fatalf("owner name = %q, want user-1", payload.Items[0].OwnerName)
	}
}

// Une installation sans extraits rend une liste vide, jamais une erreur : le
// catalogue s'ouvre avant qu'un extrait existe.
func TestTheExtractCatalogueIsEmptyWithoutAStore(t *testing.T) {
	h := Handlers{authMode: "header", ontologyStore: memory.NewOntologyObjectStore()}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/extracts", nil)
	request.Header.Set(userHeader, "user-1")
	recorder := httptest.NewRecorder()
	h.ListExtracts(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
}

// Le dossier sous la modalite est selectionnable, et c'est le seul axe qui
// separe deux fichiers de meme genre.
//
// L'ANTERION de SELENA porte des surimpressions de segmentation et, dans son
// export DICOM, les images du visualiseur web livre avec le support. Les deux
// sont des PNG : demander "les PNG d'ANTERION" rend le mobilier du visualiseur
// avec les mesures. C'est pour rendre ce niveau selectionnable que l'etude a
// ete rerangee.
func TestUnExtraitSeSelectionneSousLaModalite(t *testing.T) {
	ontologies := memory.NewOntologyObjectStore()
	objet := ontologydomain.New("user-1", "SELENA", "", "dataset", "dataset-1", "HDS-For-selena", "health-file-path-v1", []byte(`{}`))
	if err := ontologies.Create(objet); err != nil {
		t.Fatal(err)
	}
	if err := ontologies.ReplaceObjects(objet.ID, []ontologydomain.Object{
		{Path: "SELENA/S1/v1/ANTERION/SegmentOverlays/od.png", SubjectID: "S1", Visit: "v1", Modality: "ANTERION", Format: "PNG", DataType: "SegmentOverlays", SizeBytes: 10},
		{Path: "SELENA/S1/v1/ANTERION/DICOM/IHE_PDI/images/valxhtml.png", SubjectID: "S1", Visit: "v1", Modality: "ANTERION", Format: "PNG", DataType: "DICOM", SizeBytes: 20},
		{Path: "SELENA/S1/v1/ANTERION/CorneaMaps/cor.csv", SubjectID: "S1", Visit: "v1", Modality: "ANTERION", Format: "CSV", DataType: "CorneaMaps", SizeBytes: 30},
	}); err != nil {
		t.Fatal(err)
	}
	h := Handlers{authMode: "header", ontologyStore: ontologies, extractStore: memory.NewExtractStore()}

	// Par genre : le visualiseur part avec les mesures.
	parGenre := postExtract(t, h, objet.ID, map[string]any{
		"name": "par-genre", "modalities": []string{"ANTERION"}, "formats": []string{"PNG"},
	})
	var genre extractdomain.Extract
	if err := json.Unmarshal(parGenre.Body.Bytes(), &genre); err != nil {
		t.Fatal(err)
	}
	if genre.ObjectCount != 2 {
		t.Fatalf("selection par genre = %d objets, les deux PNG etaient attendus", genre.ObjectCount)
	}

	// Par dossier : seules les surimpressions.
	parDossier := postExtract(t, h, objet.ID, map[string]any{
		"name": "par-dossier", "modalities": []string{"ANTERION"}, "dataTypes": []string{"SegmentOverlays"},
	})
	if parDossier.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", parDossier.Code, parDossier.Body.String())
	}
	var dossier extractdomain.Extract
	if err := json.Unmarshal(parDossier.Body.Bytes(), &dossier); err != nil {
		t.Fatal(err)
	}
	if dossier.ObjectCount != 1 {
		t.Fatalf("selection par dossier = %d objets, une seule surimpression etait attendue", dossier.ObjectCount)
	}
	if len(dossier.DataTypes) != 1 || dossier.DataTypes[0] != "SegmentOverlays" {
		t.Fatalf("la selection doit rester lisible dans l'extrait : %v", dossier.DataTypes)
	}

	// Et deux dossiers a la fois, sans l'export DICOM.
	deux := postExtract(t, h, objet.ID, map[string]any{
		"name": "mesures-et-surimpressions", "modalities": []string{"ANTERION"},
		"dataTypes": []string{"SegmentOverlays", "CorneaMaps"},
	})
	var paire extractdomain.Extract
	if err := json.Unmarshal(deux.Body.Bytes(), &paire); err != nil {
		t.Fatal(err)
	}
	if paire.ObjectCount != 2 {
		t.Fatalf("deux dossiers = %d objets", paire.ObjectCount)
	}
}

// Une modalite sans dossier n'est pas perdue par un filtre de dossiers.
//
// L'extrait test-inria a ete declare avec OPTOS parmi ses modalites et aucun
// fichier OPTOS dedans : OPTOS pose ses images directement sous la modalite,
// le filtre demandait des dossiers, et l'intersection l'a videe en silence.
// La regle est celle qu'on lit sur le formulaire : ne rien cocher pour une
// modalite, c'est la prendre entiere - exactement comme ne cocher aucun
// patient les prend tous.
func TestLesDossiersNeResserrentQueLesModalitesQuIlsNomment(t *testing.T) {
	ontologies := memory.NewOntologyObjectStore()
	objet := ontologydomain.New("user-1", "SELENA", "", "dataset", "dataset-1", "HDS-For-selena", "health-file-path-v1", []byte(`{}`))
	if err := ontologies.Create(objet); err != nil {
		t.Fatal(err)
	}
	if err := ontologies.ReplaceObjects(objet.ID, []ontologydomain.Object{
		{Path: "SELENA/S1/v1/ANTERION/TabulatedData/basics.csv", SubjectID: "S1", Visit: "v1", Modality: "ANTERION", Format: "CSV", DataType: "TabulatedData", SizeBytes: 10},
		{Path: "SELENA/S1/v1/ANTERION/DICOM/DICOM/im.dcm", SubjectID: "S1", Visit: "v1", Modality: "ANTERION", Format: "DICOM", DataType: "DICOM", SizeBytes: 20},
		{Path: "SELENA/S1/v1/OPTOS/1.2.826.dcm", SubjectID: "S1", Visit: "v1", Modality: "OPTOS", Format: "DICOM", SizeBytes: 30},
		{Path: "SELENA/S1/v1/OPTOS/1.2.827.dcm", SubjectID: "S1", Visit: "v1", Modality: "OPTOS", Format: "DICOM", SizeBytes: 40},
	}); err != nil {
		t.Fatal(err)
	}
	h := Handlers{authMode: "header", ontologyStore: ontologies, extractStore: memory.NewExtractStore()}

	lire := func(recorder *httptest.ResponseRecorder) extractdomain.Extract {
		t.Helper()
		var item extractdomain.Extract
		if err := json.Unmarshal(recorder.Body.Bytes(), &item); err != nil {
			t.Fatalf("decode: %v - %s", err, recorder.Body.String())
		}
		return item
	}

	// Le cas de test-inria : ANTERION resserree sur son dossier, OPTOS entiere.
	melange := lire(postExtract(t, h, objet.ID, map[string]any{
		"name": "test-inria", "modalities": []string{"ANTERION", "OPTOS"},
		"dataTypes": []string{"TabulatedData"},
	}))
	if melange.ObjectCount != 3 {
		t.Fatalf("= %d objets, le fichier tabule et les deux OPTOS etaient attendus", melange.ObjectCount)
	}

	// Une modalite seule, sans dossier coche : elle vient entiere.
	entiere := lire(postExtract(t, h, objet.ID, map[string]any{
		"name": "anterion-entiere", "modalities": []string{"ANTERION"},
	}))
	if entiere.ObjectCount != 2 {
		t.Fatalf("ANTERION entiere = %d objets", entiere.ObjectCount)
	}

	// Et un dossier seul reste un dossier seul pour la modalite qui le porte.
	dossier := lire(postExtract(t, h, objet.ID, map[string]any{
		"name": "tabule-seul", "modalities": []string{"ANTERION"},
		"dataTypes": []string{"TabulatedData"},
	}))
	if dossier.ObjectCount != 1 {
		t.Fatalf("ANTERION/TabulatedData = %d objets", dossier.ObjectCount)
	}
}

// La paternite suit la source : lire la donnee de quelqu'un n'en fait pas la
// sienne, et decouper son etude non plus.
//
// Une ontologie appartenait a qui avait presse "scanner", et un extrait a qui
// l'avait declare - si bien que l'equipe proprietaire du bucket devait
// demander l'acces a la description de sa propre etude.
func TestLaPaterniteSuitLaSource(t *testing.T) {
	datasets := memory.NewDatasetStore()
	source := datasetdomain.New("createur", "HDS-For-selena", "", "hds-for-selena", "", "minio", "hds", "", "")
	source.OwnerType, source.OwnerID = "team", "equipe-ophtalmo"
	if err := datasets.Create(source); err != nil {
		t.Fatal(err)
	}
	ontologies := memory.NewOntologyObjectStore()
	lue := ontologydomain.New("quelqu-un-dautre", "SELENA", "", "dataset", source.ID, source.Name, "", []byte(`{}`))
	lue.InheritOwner(source.OwnerType, source.OwnerID)
	if lue.OwnerType != "team" || lue.OwnerID != "equipe-ophtalmo" {
		t.Fatalf("ontologie = %s/%s, celle du dataset etait attendue", lue.OwnerType, lue.OwnerID)
	}
	if lue.OwnerUserID != "quelqu-un-dautre" {
		t.Fatalf("l'auteur de la lecture doit rester nomme : %q", lue.OwnerUserID)
	}
	if err := ontologies.Create(lue); err != nil {
		t.Fatal(err)
	}
	if err := ontologies.ReplaceObjects(lue.ID, []ontologydomain.Object{
		{Path: "SELENA/S1/v1/ANTERION/TabulatedData/b.csv", SubjectID: "S1", Visit: "v1", Modality: "ANTERION", DataType: "TabulatedData", SizeBytes: 10},
	}); err != nil {
		t.Fatal(err)
	}

	// L'extrait est declare par un administrateur, parce que l'ontologie
	// appartient desormais a l'equipe et plus a qui l'a lue : c'est la
	// consequence voulue, et la seule facon d'en declarer un depuis l'exterieur
	// de l'equipe.
	h := Handlers{
		authMode: "header", ontologyStore: ontologies, extractStore: memory.NewExtractStore(),
		datasetStore: datasets, bootstrapAdminUser: "user-1",
	}
	recorder := postExtract(t, h, lue.ID, map[string]any{"name": "pour-inria", "modalities": []string{"ANTERION"}})
	if recorder.Code != http.StatusCreated {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	var extrait extractdomain.Extract
	if err := json.Unmarshal(recorder.Body.Bytes(), &extrait); err != nil {
		t.Fatal(err)
	}
	if extrait.OwnerType != "team" || extrait.OwnerID != "equipe-ophtalmo" {
		t.Fatalf("extrait = %s/%s, celui de l'ontologie etait attendu", extrait.OwnerType, extrait.OwnerID)
	}
	if extrait.OwnerUserID != "user-1" {
		t.Fatalf("l'auteur de l'extrait doit rester nomme : %q", extrait.OwnerUserID)
	}

	// Une source sans proprietaire ne deplace rien.
	orpheline := ontologydomain.New("auteur", "AUTRE", "", "dataset", "inconnu", "", "", []byte(`{}`))
	avant := orpheline.OwnerID
	orpheline.InheritOwner("", "")
	if orpheline.OwnerID != avant || orpheline.OwnerType != "user" {
		t.Fatal("une source sans proprietaire doit laisser l'objet ou il est")
	}
}

// Un extrait se prete en lecture, sans donner l'etude ni s'en dessaisir.
//
// Avant, un extrait n'etait visible que par qui voyait l'ontologie : donner
// une selection a l'INRIA voulait dire lui donner SELENA entiere, ou lui
// transferer l'extrait et le perdre. C'est la mauvaise paire de choix pour
// l'objet dont le but est d'etre plus petit que l'etude.
func TestUnExtraitSePreteEnLecture(t *testing.T) {
	ontologies := memory.NewOntologyObjectStore()
	lue := ontologydomain.New("proprietaire", "SELENA", "", "dataset", "d1", "HDS-For-selena", "", []byte(`{}`))
	if err := ontologies.Create(lue); err != nil {
		t.Fatal(err)
	}
	if err := ontologies.ReplaceObjects(lue.ID, []ontologydomain.Object{
		{Path: "SELENA/S1/v1/ANTERION/TabulatedData/b.csv", SubjectID: "S1", Visit: "v1", Modality: "ANTERION", DataType: "TabulatedData", SizeBytes: 10},
	}); err != nil {
		t.Fatal(err)
	}
	extraits := memory.NewExtractStore()
	selection := extractdomain.New("proprietaire", lue.ID, "", "pour-inria", "", nil, []string{"ANTERION"}, nil, nil, nil, []string{"TabulatedData"})
	if err := extraits.Create(selection, nil); err != nil {
		t.Fatal(err)
	}
	h := Handlers{authMode: "header", ontologyStore: ontologies, extractStore: extraits}

	lire := func(user string) int {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/extracts/"+selection.ID+"/members", nil)
		request.Header.Set(userHeader, user)
		request.SetPathValue("extractID", selection.ID)
		recorder := httptest.NewRecorder()
		h.GetExtractMembers(recorder, request)
		return recorder.Code
	}

	// Avant la permission : l'invite ne voit rien, parce qu'il ne voit pas
	// l'ontologie.
	if code := lire("inria"); code != http.StatusNotFound {
		t.Fatalf("sans permission = %d, 404 etait attendu", code)
	}

	// Le proprietaire accorde la lecture.
	accorder := httptest.NewRequest(http.MethodPut, "/api/v1/extracts/"+selection.ID+"/access/user/inria", nil)
	accorder.Header.Set(userHeader, "proprietaire")
	accorder.SetPathValue("extractID", selection.ID)
	accorder.SetPathValue("subjectType", "user")
	accorder.SetPathValue("subjectID", "inria")
	recorder := httptest.NewRecorder()
	h.SetExtractAccess(recorder, accorder)
	if recorder.Code != http.StatusOK {
		t.Fatalf("octroi = %d, %s", recorder.Code, recorder.Body.String())
	}

	// Apres : l'invite lit l'extrait, et toujours pas l'ontologie.
	if code := lire("inria"); code != http.StatusOK {
		t.Fatalf("avec permission = %d, 200 etait attendu", code)
	}
	listeOnto := httptest.NewRequest(http.MethodGet, "/api/v1/ontologies/"+lue.ID+"/completeness", nil)
	listeOnto.Header.Set(userHeader, "inria")
	listeOnto.SetPathValue("ontologyID", lue.ID)
	vueOnto := httptest.NewRecorder()
	h.GetOntologyCompleteness(vueOnto, listeOnto)
	if vueOnto.Code != http.StatusNotFound {
		t.Fatalf("l'ontologie reste fermee : %d", vueOnto.Code)
	}

	// L'invite ne peut pas re-preter ce qu'on lui a prete.
	repret := httptest.NewRequest(http.MethodPut, "/api/v1/extracts/"+selection.ID+"/access/user/ailleurs", nil)
	repret.Header.Set(userHeader, "inria")
	repret.SetPathValue("extractID", selection.ID)
	repret.SetPathValue("subjectType", "user")
	repret.SetPathValue("subjectID", "ailleurs")
	vueRepret := httptest.NewRecorder()
	h.SetExtractAccess(vueRepret, repret)
	if vueRepret.Code != http.StatusForbidden {
		t.Fatalf("un lecteur ne redistribue pas : %d", vueRepret.Code)
	}

	// Et la permission se reprend.
	retirer := httptest.NewRequest(http.MethodDelete, "/api/v1/extracts/"+selection.ID+"/access/user/inria", nil)
	retirer.Header.Set(userHeader, "proprietaire")
	retirer.SetPathValue("extractID", selection.ID)
	retirer.SetPathValue("subjectType", "user")
	retirer.SetPathValue("subjectID", "inria")
	vueRetrait := httptest.NewRecorder()
	h.DeleteExtractAccess(vueRetrait, retirer)
	if vueRetrait.Code != http.StatusNoContent {
		t.Fatalf("retrait = %d", vueRetrait.Code)
	}
	if code := lire("inria"); code != http.StatusNotFound {
		t.Fatalf("apres retrait = %d, 404 etait attendu", code)
	}
}
