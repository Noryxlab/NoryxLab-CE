package handlers

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	secretdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/secret"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/security"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Un extrait sans son dataset se monte quand meme, et le bucket ne suit pas.
//
// C'est la raison d'etre d'un extrait : donner une selection reorganisee sans
// donner le dataset dont elle est tiree. Jusqu'ici ce cas ne donnait rien du
// tout - les liens ne peuvent pas pointer dans un montage que la charge n'a
// pas - et la seule facon de voir l'extrait etait d'attacher le bucket, ce qui
// detruit la garantie qu'on voulait.
//
// Les objets arrivent donc par le remplisseur : il monte le bucket en lecture
// seule a cote, recopie les objets listes dans un cache partage, et la charge
// recoit le sous-arbre du cache. Le bucket n'est jamais monte la ou la
// personne travaille, et c'est ce que ce test verifie sur le pod.
func TestUnExtraitSansSonDatasetSeMonteSansLeBucket(t *testing.T) {
	h, runner, item := launchFixture(t, "")
	h.secretsMasterKey = testMasterKey
	accessStore := memory.NewAccessStore()
	accessStore.SetRole(item.ID, "member", access.RoleEditor)
	h.accessStore = accessStore

	// Le dataset existe, avec ses identifiants - et n'est PAS attache.
	jeu := datasetdomain.New("owner", "HDS-For-selena", "", "hds-for-selena", "", "s3", "hds",
		"https://cellar-c2.services.clever-cloud.com", "")
	jeu.CredentialName = "dataset-s3-" + jeu.ID
	jeu.CredentialUserID = "owner"
	if err := h.datasetStore.Create(jeu); err != nil {
		t.Fatal(err)
	}
	chiffre, err := security.EncryptString(testMasterKey, `{"accessKey":"ak","secretKey":"sk"}`)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.secretStore.Upsert(secretdomain.Secret{
		UserID: "owner", Name: jeu.CredentialName, ValueEncrypted: chiffre,
	}); err != nil {
		t.Fatal(err)
	}

	ontologies := memory.NewOntologyObjectStore()
	onto := ontologydomain.New("owner", "Selena", "", "dataset", jeu.ID, jeu.Name,
		"health-file-path-v1", json.RawMessage(`{}`))
	if err := ontologies.Create(onto); err != nil {
		t.Fatal(err)
	}
	h.ontologyStore = ontologies

	extraits := memory.NewExtractStore()
	extrait := extractdomain.Extract{ID: "e-selena", OntologyID: onto.ID, Name: "selena_extract_mri",
		OwnerUserID: "owner", OwnerType: ownerUser, OwnerID: "owner"}
	if err := extraits.Create(extrait, []extractdomain.Member{
		{ExtractID: extrait.ID, Path: "SUJ-01/20260218/MRI/a.dcm", SubjectID: "SUJ-01", Visit: "20260218", Modality: "MRI"},
	}); err != nil {
		t.Fatal(err)
	}
	h.extractStore = extraits
	if err := h.projectResourceStore.AttachExtract(item.ID, extrait.ID); err != nil {
		t.Fatal(err)
	}

	if recorder := launch(t, h, item.ID, "", ""); recorder.Code >= 300 {
		t.Fatalf("le lancement a echoue: %d %s", recorder.Code, recorder.Body.String())
	}
	if len(runner.pods) != 1 {
		t.Fatalf("attendu un pod, recu %d", len(runner.pods))
	}
	pod := runner.pods[0]

	// L'arbre arrive en volume, a la place des liens.
	arbre := false
	for _, v := range pod.Volumes {
		if v.MountPath == "/extracts" {
			arbre = true
			if !v.ReadOnly {
				t.Error("l'arbre d'extraits doit etre monte en lecture seule")
			}
		}
		if strings.HasPrefix(v.MountPath, "/datasets/") {
			t.Errorf("le bucket ne doit pas etre monte sur la charge: %s", v.MountPath)
		}
	}
	if !arbre {
		t.Fatalf("l'arbre d'extraits n'est pas monte: %+v", pod.Volumes)
	}

	// Et le remplisseur, lui, le monte - en lecture seule.
	if pod.Sidecar == nil {
		t.Fatal("le remplisseur doit tourner a cote de la charge")
	}
	source := false
	for _, v := range pod.Sidecar.Volumes {
		if strings.HasPrefix(v.MountPath, "/datasets/") {
			source = true
			if !v.ReadOnly {
				t.Errorf("le remplisseur ne doit jamais pouvoir ecrire dans la source: %s", v.MountPath)
			}
		}
	}
	if !source {
		t.Fatalf("le remplisseur n'a pas la source a copier: %+v", pod.Sidecar.Volumes)
	}

	// Et le bootstrap ne construit pas de liens par-dessus un volume en
	// lecture seule.
	if strings.Contains(strings.Join(pod.Args, " "), "building extract links") {
		t.Error("le bootstrap ne doit pas batir de liens quand l'arbre est monte")
	}
}

// Et le cas ordinaire ne change rien : dataset attache, aucun remplissage.
//
// Teste sur le montage et non sur le pod : faire entrer un dataset dans la
// liste des attaches demande toute la configuration MinIO du deploiement, et
// l'invariant qui compte ici est plus simple - si le bucket est deja monte
// la ou la charge le voit, rien n'est a copier.
func TestUnDatasetAttacheNeDeclencheAucunRemplissage(t *testing.T) {
	h, _, item := launchFixture(t, "")

	jeu := datasetdomain.New("member", "verif", "", "noryx-ds-verif", "", "", "non-hds", "", "")
	if err := h.datasetStore.Create(jeu); err != nil {
		t.Fatal(err)
	}
	ontologies := memory.NewOntologyObjectStore()
	onto := ontologydomain.New("member", "V", "", "dataset", jeu.ID, jeu.Name, "health-file-path-v1", json.RawMessage(`{}`))
	if err := ontologies.Create(onto); err != nil {
		t.Fatal(err)
	}
	h.ontologyStore = ontologies
	extraits := memory.NewExtractStore()
	extrait := extractdomain.Extract{ID: "e-v", OntologyID: onto.ID, Name: "v",
		OwnerUserID: "member", OwnerType: ownerUser, OwnerID: "member"}
	if err := extraits.Create(extrait, []extractdomain.Member{
		{ExtractID: extrait.ID, Path: "a/b.dcm", SubjectID: "a", Visit: "v", Modality: "m"},
	}); err != nil {
		t.Fatal(err)
	}
	h.extractStore = extraits
	if err := h.projectResourceStore.AttachExtract(item.ID, extrait.ID); err != nil {
		t.Fatal(err)
	}

	montage := h.extractMountFor(item.ID, []workspaceAttachedDataset{
		{ID: jeu.ID, Name: jeu.Name, Bucket: jeu.Bucket},
	})
	if montage.Manifest == "" {
		t.Fatalf("l'extrait devait se monter: %+v", montage)
	}
	if len(montage.ToFill) != 0 {
		t.Fatalf("rien n'est a remplir quand le bucket est attache: %+v", montage.ToFill)
	}
}

// Les quatre charges recoivent une selection sans son bucket, de la meme facon.
//
// Verifie sur le texte, pour la meme raison que le test de politique a cote :
// ce qui compte est qu'aucune des quatre ne reste en arriere. Une selection qui
// se monte dans un workspace et pas dans un job donnerait une garantie qui
// tient pendant qu'on tape et cesse des qu'un calcul tourne - c'est exactement
// ce que l'ADR-044 avait eu a corriger sur la frontiere elle-meme.
func TestLesQuatreChargesRemplissentUneSelectionSansSonBucket(t *testing.T) {
	for _, cas := range []struct{ fichier, conteneur string }{
		// Le workspace l'a a cote : on commence a travailler sur le premier
		// fichier plutot que sur le dernier.
		{"workspaces.go", "Sidecar:"},
		// Les trois autres l'ont avant : une demi-selection est une autre
		// etude, et une mauvaise reponse servie vite reste mauvaise.
		{"jobs.go", "Init:"},
		{"cronjobs.go", "Init:"},
		{"apps.go", "Init:"},
	} {
		source, err := os.ReadFile(cas.fichier)
		if err != nil {
			t.Fatal(err)
		}
		texte := string(source)
		if !strings.Contains(texte, "h.extractFillFor(") {
			t.Errorf("%s ne remplit pas une selection dont le bucket n'est pas attache", cas.fichier)
		}
		if !strings.Contains(texte, cas.conteneur+"                    extractFiller,") &&
			!strings.Contains(texte, cas.conteneur+" extractFiller,") &&
			!regexp.MustCompile(regexp.QuoteMeta(cas.conteneur)+`\s+extractFiller,`).MatchString(texte) {
			t.Errorf("%s ne lance pas le remplisseur en %s", cas.fichier, strings.TrimSuffix(cas.conteneur, ":"))
		}
		// Et aucune ne batit de liens par-dessus un montage en lecture seule.
		if strings.Contains(texte, "extractBootstrapLines(") &&
			!strings.Contains(texte, "!montage.TreeMounted") &&
			!strings.Contains(texte, "extractTree == nil") {
			t.Errorf("%s batirait des liens par-dessus l'arbre monte", cas.fichier)
		}
	}
}

// Et une app relancee ne reperd pas sa selection.
func TestUneAppRelanceeGardeSaSelectionRemplie(t *testing.T) {
	source, err := os.ReadFile("apps.go")
	if err != nil {
		t.Fatal(err)
	}
	// Deux appels : la creation et la relance. Un seul voudrait dire que l'une
	// des deux reperd l'arbre, ce qui est le defaut corrige le 2026-10-12 sur
	// le manifeste, une couche plus bas.
	if n := strings.Count(string(source), "h.extractFillFor("); n < 2 {
		t.Fatalf("la creation et la relance doivent remplir toutes les deux, trouve %d appel(s)", n)
	}
}
