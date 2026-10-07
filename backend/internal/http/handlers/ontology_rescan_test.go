package handlers

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

func ontologiePour(t *testing.T, proprietaire, source, profil string) (Handlers, ontologydomain.Ontology) {
	t.Helper()
	store := memory.NewOntologyObjectStore()
	item := ontologydomain.New(proprietaire, "PREMYOM1000", "", "dataset", source, "HDS-For", profil, []byte(`{"summary":{"objects":10}}`))
	if err := store.Create(item); err != nil {
		t.Fatal(err)
	}
	return Handlers{ontologyStore: store}, item
}

// Un second scan de la meme source rafraichit l'ontologie existante.
//
// Le scan inserait toujours : scanner deux fois un dataset laissait deux
// ontologies portant le meme nom sur le meme bucket, qu'on ne distinguait que
// par leurs dates. C'est l'origine des deux PREMYOM1000 sur EMSE.
func TestUnSecondScanRafraichitAuLieuDeDupliquer(t *testing.T) {
	h, existante := ontologiePour(t, "stef", "dataset-1", "health-file-path-v1")
	identite := auth.Identity{Username: "stef", Subject: "stef"}

	choisie, trouvee, err := h.ontologyToRefresh(identite, ontologyManifest{
		SourceType: "dataset", SourceID: "dataset-1", InferenceProfile: "health-file-path-v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !trouvee {
		t.Fatal("un re-scan de la meme source doit retrouver l'ontologie a rafraichir")
	}
	if choisie.ID != existante.ID {
		t.Fatalf("ontologie retenue = %s, attendu %s", choisie.ID, existante.ID)
	}

	// Et le manifeste est bien remplace, l'objet garde son identite.
	nouveau := []byte(`{"summary":{"objects":4026}}`)
	if err := h.ontologyStore.ReplaceManifest(choisie.ID, nouveau, "stef"); err != nil {
		t.Fatal(err)
	}
	relue, _, err := h.ontologyStore.GetByID(existante.ID)
	if err != nil {
		t.Fatal(err)
	}
	var resume struct {
		Summary struct{ Objects int } `json:"summary"`
	}
	if err := json.Unmarshal(relue.Manifest, &resume); err != nil {
		t.Fatal(err)
	}
	if resume.Summary.Objects != 4026 {
		t.Fatalf("objets apres rafraichissement = %d, attendu 4026", resume.Summary.Objects)
	}
	if relue.Name != "PREMYOM1000" {
		t.Fatalf("le nom a bouge : %q", relue.Name)
	}
}

// Une autre source reste une autre ontologie. Un autre profil, non.
//
// Ce test exigeait l'inverse pour le profil, et la raison etait bonne : deux
// lectures du meme bucket sont deux descriptions, et ecraser l'une avec l'autre
// ferait disparaitre un travail sans le dire. Ce qu'elle manquait, c'est ce que
// la branche « pas trouve » faisait ensuite - elle creait une jumelle. Donc le
// travail n'etait pas protege, il etait duplique, et EMSE a porte deux
// PREMYOM1000 sur un bucket pendant un mois, 31 sujets et 32, sans rien pour
// dire laquelle faisait foi.
//
// Une source, une ontologie - desormais garanti par un index unique sur
// (source_type, source_id), donc l'ancienne attente n'est plus seulement
// indesirable, elle est impossible a satisfaire. Ce qu'un rescan avec une autre
// lecture change se lit dans la regle de lecture que chaque photographie
// enregistre (ADR-040), qui est precisement ce a quoi elle sert : deux
// photographies du meme bucket different soit parce que la donnee a bouge, soit
// parce que la lecture a change, et le record doit pouvoir le dire.
func TestUneAutreSourceResteUneAutreOntologieMaisPasUnAutreProfil(t *testing.T) {
	h, _ := ontologiePour(t, "stef", "dataset-1", "health-file-path-v1")
	identite := auth.Identity{Username: "stef", Subject: "stef"}

	if _, trouvee, _ := h.ontologyToRefresh(identite, ontologyManifest{
		SourceType: "dataset", SourceID: "dataset-2", InferenceProfile: "health-file-path-v1",
	}); trouvee {
		t.Error("une autre source ne doit pas etre rafraichie")
	}
	if _, trouvee, _ := h.ontologyToRefresh(identite, ontologyManifest{
		SourceType: "dataset", SourceID: "dataset-1", InferenceProfile: "autre-profil-v1",
	}); !trouvee {
		t.Error("un autre profil sur la meme source doit rafraichir, pas dupliquer")
	}
	if _, trouvee, _ := h.ontologyToRefresh(identite, ontologyManifest{
		SourceType: "datasource", SourceID: "dataset-1", InferenceProfile: "health-file-path-v1",
	}); trouvee {
		t.Error("un autre type de source ne doit pas etre rafraichi")
	}
}

// Et on ne rafraichit jamais l'ontologie de quelqu'un d'autre.
//
// Rafraichir ce qu'on ne voit pas serait modifier l'objet d'autrui par un
// scan : la personne scanne, et une description qu'elle n'a jamais vue change.
func TestUnScanNeRafraichitPasUneOntologieInvisible(t *testing.T) {
	h, _ := ontologiePour(t, "alice", "dataset-1", "health-file-path-v1")
	if _, trouvee, _ := h.ontologyToRefresh(auth.Identity{Username: "bob", Subject: "bob"}, ontologyManifest{
		SourceType: "dataset", SourceID: "dataset-1", InferenceProfile: "health-file-path-v1",
	}); trouvee {
		t.Error("bob ne doit pas rafraichir l'ontologie d'alice")
	}
}

// La plus recemment mise a jour quand il y en a plusieurs : c'est celle que
// les ecrans montrent en premier, donc celle que la personne croit scanner.
func TestLeRafraichissementChoisitLaPlusRecente(t *testing.T) {
	store := memory.NewOntologyObjectStore()
	vieille := ontologydomain.New("stef", "PREMYOM1000", "", "dataset", "dataset-1", "HDS-For", "health-file-path-v1", []byte(`{}`))
	vieille.UpdatedAt = time.Now().UTC().Add(-48 * time.Hour)
	recente := ontologydomain.New("stef", "PREMYOM1000", "", "dataset", "dataset-1", "HDS-For", "health-file-path-v1", []byte(`{}`))
	recente.UpdatedAt = time.Now().UTC()
	for _, item := range []ontologydomain.Ontology{vieille, recente} {
		if err := store.Create(item); err != nil {
			t.Fatal(err)
		}
	}
	h := Handlers{ontologyStore: store}
	choisie, trouvee, err := h.ontologyToRefresh(auth.Identity{Username: "stef", Subject: "stef"}, ontologyManifest{
		SourceType: "dataset", SourceID: "dataset-1", InferenceProfile: "health-file-path-v1",
	})
	if err != nil || !trouvee {
		t.Fatalf("trouvee=%v err=%v", trouvee, err)
	}
	if choisie.ID != recente.ID {
		t.Fatal("le rafraichissement doit porter sur la plus recemment mise a jour")
	}
}
