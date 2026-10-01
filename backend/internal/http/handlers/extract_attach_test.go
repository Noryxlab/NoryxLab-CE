package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Un extrait se rattache a plusieurs projets, et n en porte plus aucun.
//
// Il portait le projet qui le monterait, decide a sa declaration : un seul,
// pour toujours, et faux des qu on voulait le monter ailleurs. C etait le meme
// prealable que l ontologie a abandonne, une couche plus bas.
func TestUnExtraitSeMonteDansPlusieursProjets(t *testing.T) {
	liens := memory.NewProjectResourceStore()

	for _, projet := range []string{"p1", "p2"} {
		if err := liens.AttachExtract(projet, "e1"); err != nil {
			t.Fatalf("rattachement a %s : %v", projet, err)
		}
	}
	for _, projet := range []string{"p1", "p2"} {
		ids, err := liens.ListProjectExtractIDs(projet)
		if err != nil || len(ids) != 1 || ids[0] != "e1" {
			t.Errorf("%s voit %v (%v)", projet, ids, err)
		}
	}
	projets, err := liens.ListExtractProjectIDs("e1")
	if err != nil || len(projets) != 2 {
		t.Errorf("l extrait est monte dans %v (%v), attendu deux projets", projets, err)
	}
}

// Detacher ne detruit pas : une selection gelee survit aux projets qui s en
// sont servis.
func TestDetacherNeDetruitPasLExtrait(t *testing.T) {
	liens := memory.NewProjectResourceStore()
	_ = liens.AttachExtract("p1", "e1")
	_ = liens.AttachExtract("p2", "e1")

	if err := liens.DetachExtract("p1", "e1"); err != nil {
		t.Fatal(err)
	}
	if ids, _ := liens.ListProjectExtractIDs("p1"); len(ids) != 0 {
		t.Errorf("p1 voit encore %v", ids)
	}
	// L autre projet ne doit pas avoir bouge.
	if ids, _ := liens.ListProjectExtractIDs("p2"); len(ids) != 1 {
		t.Errorf("p2 a perdu son extrait : %v", ids)
	}
}

// Et rattacher deux fois ne cree pas deux liens.
func TestRattacherDeuxFoisNeDoublePas(t *testing.T) {
	liens := memory.NewProjectResourceStore()
	_ = liens.AttachExtract("p1", "e1")
	_ = liens.AttachExtract("p1", "e1")
	if ids, _ := liens.ListProjectExtractIDs("p1"); len(ids) != 1 {
		t.Errorf("%d liens pour un seul rattachement : %v", len(ids), ids)
	}
}
