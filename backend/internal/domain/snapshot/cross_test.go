package snapshot

import "testing"

func TestUnMemeETagEstUneCopieAffirmee(t *testing.T) {
	regulated := []Entry{entry("SELENA-01-003/visit1/oct.e2e", "abc123", 2048)}
	open := []Entry{
		entry("notes.md", "ffff", 12),
		entry("recup/oct.e2e", "ABC123", 2048),
	}
	found := CrossMatch(regulated, open)
	if len(found) != 1 {
		t.Fatalf("une copie attendue, %d trouvee(s): %+v", len(found), found)
	}
	if found[0].Confidence != CopyIdentical {
		t.Fatalf("confiance %q, attendu %q", found[0].Confidence, CopyIdentical)
	}
	if found[0].RegulatedPath != "SELENA-01-003/visit1/oct.e2e" || found[0].OpenPath != "recup/oct.e2e" {
		t.Fatalf("les deux cotes doivent etre nommes: %+v", found[0])
	}
}

// Le cas qui compte : une copie FUSE d'un gros fichier part en plusieurs
// parties, donc son ETag ne retombe jamais sur celui de la source.
func TestUnGrosFichierRecopieEstTrouveParNomEtTaille(t *testing.T) {
	regulated := []Entry{entry("cohorte/PREMYOM1000/sujet-12.nii.gz", "1-aaaa", 300<<20)}
	open := []Entry{entry("perso/sujet-12.nii.gz", "42-bbbb", 300<<20)}
	found := CrossMatch(regulated, open)
	if len(found) != 1 || found[0].Confidence != CopyProbable {
		t.Fatalf("une copie probable attendue, obtenu %+v", found)
	}
	if found[0].Size != 300<<20 {
		t.Fatalf("taille %d", found[0].Size)
	}
}

func TestLesPetitsFichiersNeSeSoupconnentPasSurLeNom(t *testing.T) {
	regulated := []Entry{entry("cohorte/README.md", "aaa", 120)}
	open := []Entry{entry("projet/README.md", "bbb", 120)}
	if found := CrossMatch(regulated, open); len(found) != 0 {
		t.Fatalf("aucun soupcon attendu sur un petit fichier, obtenu %+v", found)
	}
}

func TestUnMemeNomDeTailleDifferenteNEstPasUneCopie(t *testing.T) {
	regulated := []Entry{entry("cohorte/sujet-12.nii.gz", "1-aaaa", 300<<20)}
	open := []Entry{entry("perso/sujet-12.nii.gz", "1-bbbb", 299<<20)}
	if found := CrossMatch(regulated, open); len(found) != 0 {
		t.Fatalf("une taille differente n'est pas une copie, obtenu %+v", found)
	}
}

func TestUnETagVideNeRapprocheRien(t *testing.T) {
	regulated := []Entry{entry("cohorte/a.dcm", "", 10)}
	open := []Entry{entry("perso/b.dcm", "", 10)}
	if found := CrossMatch(regulated, open); len(found) != 0 {
		t.Fatalf("deux ETag vides ne sont pas une egalite, obtenu %+v", found)
	}
}

func TestLOrdreEstStableEtLesBordsSontCalmes(t *testing.T) {
	regulated := []Entry{entry("c/x", "e1", 4096), entry("c/y", "e2", 8192)}
	open := []Entry{entry("z/second", "e2", 8192), entry("a/premier", "e1", 4096)}
	found := CrossMatch(regulated, open)
	if len(found) != 2 {
		t.Fatalf("deux copies attendues, obtenu %+v", found)
	}
	if found[0].OpenPath != "a/premier" || found[1].OpenPath != "z/second" {
		t.Fatalf("ordre instable: %+v", found)
	}
	if got := CrossMatch(nil, open); len(got) != 0 {
		t.Fatalf("rien de reglemente, rien a dire: %+v", got)
	}
	if got := CrossMatch(regulated, nil); len(got) != 0 {
		t.Fatalf("rien en face, rien a dire: %+v", got)
	}
}
