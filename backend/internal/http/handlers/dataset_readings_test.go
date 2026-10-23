package handlers

import (
	"encoding/json"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Retirer un dataset emporte ce qu'on en a lu.
//
// Le 19/10/2026, retirer les trois datasets HDS du bac a sable a laisse
// `PREMYOM1000` pointer sur un dataset disparu et `anterion-3-sujets` nommer
// trois sujets. L'acces avait ete retire partout sauf dans les deux lignes
// qui disaient quels patients avaient ete selectionnes.

// L'interface est large et ce code n'en touche que deux methodes : on
// l'incorpore et on redefinit celles-la.
type ontologiesFactices struct {
	store.OntologyStore
	items     []ontology.Ontology
	supprimes []string
}

func (f *ontologiesFactices) ListAll() ([]ontology.Ontology, error) { return f.items, nil }
func (f *ontologiesFactices) Delete(id string) error {
	f.supprimes = append(f.supprimes, id)
	return nil
}

func TestRetirerUnDatasetEmporteSesOntologiesEtLeursExtraits(t *testing.T) {
	lue := ontology.New("stef", "PREMYOM1000", "", "dataset", "ds-hds-for", "HDS-For", "", json.RawMessage(`{}`))
	ailleurs := ontology.New("stef", "AUTRE", "", "dataset", "ds-autre", "Autre", "", json.RawMessage(`{}`))
	ontologies := &ontologiesFactices{items: []ontology.Ontology{lue, ailleurs}}

	extraits := memory.NewExtractStore()
	selection := extract.New("stef", lue.ID, "p1", "anterion-3-sujets", "",
		[]string{"PREMYOM1000-0023"}, []string{"ANTERION"}, nil, nil, nil, nil)
	if err := extraits.Create(selection, nil); err != nil {
		t.Fatal(err)
	}
	intacte := extract.New("stef", ailleurs.ID, "p1", "autre-extrait", "", nil, nil, nil, nil, nil, nil)
	if err := extraits.Create(intacte, nil); err != nil {
		t.Fatal(err)
	}

	h := Handlers{ontologyStore: ontologies, extractStore: extraits}
	retires, err := h.deleteDatasetReadings("ds-hds-for")
	if err != nil {
		t.Fatal(err)
	}
	if retires != 2 {
		t.Errorf("%d ligne(s) retiree(s), attendu 2 (une ontologie, un extrait)", retires)
	}
	if len(ontologies.supprimes) != 1 || ontologies.supprimes[0] != lue.ID {
		t.Errorf("ontologies supprimees = %v ; seule celle lue sur ce dataset part", ontologies.supprimes)
	}
	restants, _ := extraits.ListByProject("p1")
	if len(restants) != 1 || restants[0].Name != "autre-extrait" {
		t.Errorf("extraits restants = %v ; celui d'une autre ontologie reste", restants)
	}
}

func TestUnDatasetSansLectureNeRetireRien(t *testing.T) {
	ontologies := &ontologiesFactices{}
	h := Handlers{ontologyStore: ontologies, extractStore: memory.NewExtractStore()}
	if retires, err := h.deleteDatasetReadings("ds-seul"); err != nil || retires != 0 {
		t.Errorf("retires = %d, err = %v", retires, err)
	}
}
