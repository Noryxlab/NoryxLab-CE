package handlers

import (
	"encoding/json"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
)

// Une ontologie enregistre la regle qui l'a produite, pas son nom.
//
// C'est la seconde decision de l'ADR-040, restee ouverte : un nom de profil
// pointe vers quelque chose qui a pu changer depuis, donc deux photographies
// du meme bucket differaient sans que rien puisse dire si la donnee avait
// bouge ou si la lecture avait change.
func TestLaRegleAppliqueeEstEnregistree(t *testing.T) {
	declaree := describeReading(&dataset.PathLayout{SubjectLevel: 1, VisitLevel: 2, ModalityLevel: 3})
	if declaree.Source != "declared" || declaree.SubjectLevel != 1 {
		t.Fatalf("regle declaree = %+v", declaree)
	}
	// Sans noms declares, les defauts sont generiques : la plateforme est aussi
	// vendue a des banques, et « sujet » sur une table d'operations est une
	// demo qui perd la salle.
	if declaree.Description != "level 1 = entity, level 2 = period, level 3 = category" {
		t.Fatalf("description = %q", declaree.Description)
	}
	// Et les mots du metier sont enregistres avec la regle, pour la meme raison
	// que la regle l'est : un nom change ensuite ferait decrire une vieille
	// photographie dans un vocabulaire avec lequel elle n'a jamais ete lue.
	clinique := describeReading(&dataset.PathLayout{
		SubjectLevel: 1, VisitLevel: 2, ModalityLevel: 3,
		SubjectName: "patient", VisitName: "visite", ModalityName: "modalite"})
	if clinique.SubjectName != "patient" || clinique.VisitName != "visite" {
		t.Fatalf("noms enregistres = %+v", clinique)
	}
	if clinique.Description != "level 1 = patient, level 2 = visite, level 3 = modalite" {
		t.Fatalf("description = %q", clinique.Description)
	}

	// Et l'absence de regle est une reponse, pas un vide : c'est la regle
	// compilee qui s'applique, et il faut pouvoir le dire.
	defaut := describeReading(nil)
	if defaut.Source != "default" || defaut.Description == "" {
		t.Fatalf("regle par defaut = %+v", defaut)
	}
}

// Un rescan dit laquelle des deux choses a change.
//
// Un compte de sujets qui passe de 2 a 31 est soit l'etude qui recrute, soit
// une regle que quelqu'un vient d'editer - et les deux appellent des reactions
// opposees. Donner les chiffres sans la cause laisse deviner, et la devinette
// est en general "la plateforme est cassee".
func TestUnRescanDitSiLaLectureAChange(t *testing.T) {
	avant := ontologyManifest{
		ReadingRule: describeReading(nil),
		Summary:     ontologySummary{Objects: 4023, Subjects: 2},
	}
	brut, _ := json.Marshal(avant)

	// Meme lecture : seuls les chiffres parlent.
	memeLecture := compareOntologyScans(brut, ontologyManifest{
		ReadingRule: describeReading(nil),
		Summary:     ontologySummary{Objects: 4100, Subjects: 2},
	})
	if memeLecture == nil || memeLecture.ReadingChanged {
		t.Fatalf("la lecture n'a pas change : %+v", memeLecture)
	}
	if memeLecture.PreviousObjects != 4023 || memeLecture.CurrentObjects != 4100 {
		t.Fatalf("chiffres = %+v", memeLecture)
	}

	// Lecture changee : c'est la premiere chose a regarder.
	autreLecture := compareOntologyScans(brut, ontologyManifest{
		ReadingRule: describeReading(&dataset.PathLayout{SubjectLevel: 1, VisitLevel: 2, ModalityLevel: 3}),
		Summary:     ontologySummary{Objects: 3908, Subjects: 31},
	})
	if autreLecture == nil || !autreLecture.ReadingChanged {
		t.Fatalf("la lecture a change : %+v", autreLecture)
	}
	if autreLecture.PreviousSubjects != 2 || autreLecture.CurrentSubjects != 31 {
		t.Fatalf("sujets = %+v", autreLecture)
	}
}

// Un manifeste anterieur a cette decision ne porte aucune regle : on ne peut
// pas affirmer que la lecture a change, seulement qu'on ne sait pas.
//
// Dire "la lecture a change" a tort enverrait chercher une regle que personne
// n'a editee.
func TestUnAncienManifesteNAffirmeRien(t *testing.T) {
	ancien, _ := json.Marshal(ontologyManifest{Summary: ontologySummary{Objects: 100, Subjects: 1}})
	diff := compareOntologyScans(ancien, ontologyManifest{
		ReadingRule: describeReading(&dataset.PathLayout{SubjectLevel: 1}),
		Summary:     ontologySummary{Objects: 120, Subjects: 2},
	})
	if diff == nil {
		t.Fatal("les chiffres restent comparables")
	}
	if diff.ReadingChanged {
		t.Fatal("sans regle enregistree avant, on ne peut pas dire que la lecture a change")
	}
}

// Et un manifeste illisible ne produit aucune comparaison, plutot qu'une
// fausse.
func TestUnManifesteIllisibleNeCompareRien(t *testing.T) {
	if compareOntologyScans([]byte("{pas du json"), ontologyManifest{}) != nil {
		t.Fatal("une comparaison fausse est pire qu'aucune")
	}
	if compareOntologyScans(nil, ontologyManifest{}) != nil {
		t.Fatal("sans manifeste anterieur, rien a comparer")
	}
}
