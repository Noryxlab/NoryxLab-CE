package handlers

import (
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Une source, une ontologie.
//
// EMSE a porte deux PREMYOM1000 sur un seul bucket pendant un mois, 31 et 32
// sujets, et rien ne disait laquelle faisait foi. Deux chemins y menaient et
// ces tests les ferment tous les deux.

func identiteDe(nom string) auth.Identity {
	return auth.Identity{Username: nom, Subject: nom}
}

func ontologieSur(id, source, profil string) ontology.Ontology {
	return ontology.Ontology{
		ID: id, Name: "PREMYOM1000", OwnerUserID: "autre", OwnerType: "user", OwnerID: "autre",
		SourceType: "dataset", SourceID: source, InferenceProfile: profil,
		Manifest: []byte(`{}`), UpdatedAt: time.Now().UTC(),
	}
}

// Premier chemin : un profil d'inference different. Il separait deux
// ontologies, donc un rescan avec un autre profil creait une jumelle.
func TestUnProfilDifferentNeCreePlusUneJumelle(t *testing.T) {
	store := memory.NewOntologyObjectStore()
	if err := store.Create(ontologieSur("onto-1", "ds-1", "health-file-path-v1")); err != nil {
		t.Fatal(err)
	}
	h := Handlers{ontologyStore: store}
	// Le proprietaire rescanne avec un autre profil : il doit retrouver la
	// meme ontologie, pas en creer une seconde.
	identite := identiteDe("autre")
	trouvee, refresh, err := h.ontologyToRefresh(identite,
		ontologyManifest{SourceType: "dataset", SourceID: "ds-1", InferenceProfile: "autre-profil"})
	if err != nil {
		t.Fatal(err)
	}
	if !refresh || trouvee.ID != "onto-1" {
		t.Fatalf("rafraichissement = %v, ontologie = %q : un autre profil doit rafraichir, pas dupliquer", refresh, trouvee.ID)
	}
}

// Second chemin : une ontologie que l'appelant ne peut pas voir. Le
// rafraichissement l'ignore a juste titre - rafraichir l'objet de quelqu'un
// d'autre par un scan serait pire - mais le scan creait une jumelle a la place.
func TestUneOntologieInvisibleEstQuandMemeTrouveeSurLaSource(t *testing.T) {
	store := memory.NewOntologyObjectStore()
	if err := store.Create(ontologieSur("onto-1", "ds-1", "health-file-path-v1")); err != nil {
		t.Fatal(err)
	}
	h := Handlers{ontologyStore: store}

	// Un tiers ne la voit pas...
	tiers := identiteDe("quelqu-un-dautre")
	if _, refresh, _ := h.ontologyToRefresh(tiers, ontologyManifest{SourceType: "dataset", SourceID: "ds-1"}); refresh {
		t.Fatal("un tiers a pu rafraichir une ontologie qu'il ne voit pas")
	}
	// ... mais la plateforme sait qu'elle existe, et c'est ce qui empeche la
	// jumelle.
	autre, existe, err := h.ontologyOverSource(ontologyManifest{SourceType: "dataset", SourceID: "ds-1"})
	if err != nil {
		t.Fatal(err)
	}
	if !existe || autre.ID != "onto-1" {
		t.Fatalf("existe=%v id=%q : la source est deja decrite", existe, autre.ID)
	}
}

// Et une source libre reste libre : la verification ne doit pas bloquer le
// premier scan.
func TestUneSourceLibreResteLibre(t *testing.T) {
	h := Handlers{ontologyStore: memory.NewOntologyObjectStore()}
	if _, existe, err := h.ontologyOverSource(ontologyManifest{SourceType: "dataset", SourceID: "ds-neuf"}); err != nil || existe {
		t.Fatalf("existe=%v err=%v : rien ne decrit encore cette source", existe, err)
	}
	// Et un manifeste sans source ne bloque rien non plus.
	if _, existe, _ := h.ontologyOverSource(ontologyManifest{SourceType: "dataset"}); existe {
		t.Fatal("un manifeste sans source a declenche la verification")
	}
}
