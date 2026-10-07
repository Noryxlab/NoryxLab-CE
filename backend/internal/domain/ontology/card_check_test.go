package ontology

import (
	"testing"
	"time"
)

func entier(v int) *int    { return &v }
func grand(v int64) *int64 { return &v }
func oui(v bool) *bool     { return &v }

func trouve(checks []Check, champ string) Check {
	for _, c := range checks {
		if c.Field == champ {
			return c
		}
	}
	return Check{Field: champ, Verdict: "ABSENT"}
}

// Une declaration juste est confirmee, et la mesure est montree a cote d'elle.
func TestUneDeclarationJusteEstConfirmee(t *testing.T) {
	card := &Card{Claims: Claims{Subjects: entier(32), Objects: grand(24604),
		Modalities: []string{"DICOM", "OCT"}, FirstVisit: "20240115"}}
	m := Measured{Subjects: 32, Objects: 24604, Modalities: []string{"oct", "dicom"},
		FirstVisit: "20240115", Method: "path scan", At: time.Now()}
	for _, champ := range []string{"subjects", "objects", "modalities", "firstVisit"} {
		if got := trouve(CheckCard(card, m), champ); got.Verdict != VerdictAgrees {
			t.Errorf("%s : verdict %q, attendu %q (%+v)", champ, got.Verdict, VerdictAgrees, got)
		}
	}
}

// Un ecart est rapporte, jamais corrige : la declaration reste telle quelle.
//
// Remplacer la parole de quelqu'un par une mesure detruit la seule preuve
// qu'ils etaient en desaccord - et c'est ca le fait interessant, pas le nombre.
func TestUnEcartEstRapporteJamaisCorrige(t *testing.T) {
	card := &Card{Claims: Claims{Subjects: entier(34)}}
	checks := CheckCard(card, Measured{Subjects: 32, Method: "path scan"})
	c := trouve(checks, "subjects")
	if c.Verdict != VerdictDiffers {
		t.Fatalf("verdict %q, attendu %q", c.Verdict, VerdictDiffers)
	}
	if c.Declared != "34" || c.Measured != "32" {
		t.Fatalf("les deux cotes doivent etre montres : declare=%q mesure=%q", c.Declared, c.Measured)
	}
	if *card.Claims.Subjects != 34 {
		t.Fatalf("la declaration a ete reecrite : %d", *card.Claims.Subjects)
	}
	if c.Note == "" {
		t.Fatal("un ecart sans explication envoie chercher au mauvais endroit")
	}
}

// Ce que personne n'a declare se dit, au lieu d'etre omis : un lecteur qui ne
// voit pas le champ ne peut pas le distinguer d'un champ qui est passe.
func TestLeNonDeclareSeDit(t *testing.T) {
	checks := CheckCard(nil, Measured{Subjects: 32, Method: "path scan"})
	c := trouve(checks, "subjects")
	if c.Verdict != VerdictUndeclared {
		t.Fatalf("verdict %q, attendu %q", c.Verdict, VerdictUndeclared)
	}
	if c.Measured != "32" {
		t.Fatalf("la mesure doit rester visible meme sans declaration : %q", c.Measured)
	}
}

// La pseudonymisation declaree reste "non verifiee" tant qu'aucun scan de
// structure n'a lu les champs techniques. C'est la reponse honnete, et la
// raison d'etre du champ : l'enquete du 05/10/2026 n'avait nulle part pour
// ecrire que 3 592 en-tetes DICOM n'avaient pas ete regardes.
func TestLaPseudonymisationResteNonVerifieeSansScanDeStructure(t *testing.T) {
	card := &Card{Claims: Claims{Pseudonymised: oui(true)}}
	c := trouve(CheckCard(card, Measured{Subjects: 32}), "pseudonymised")
	if c.Verdict != VerdictUnverified {
		t.Fatalf("verdict %q, attendu %q", c.Verdict, VerdictUnverified)
	}
	if c.Note == "" {
		t.Fatal("il faut dire pourquoi ce n'est pas verifie")
	}
}

// Et une fois le scan passe, la combinaison qui compte : declare pseudonymise,
// identifiants trouves.
func TestPseudonymisationDeclareeEtIdentifiantsTrouves(t *testing.T) {
	card := &Card{Claims: Claims{Pseudonymised: oui(true)}}
	vus := true
	m := Measured{IdentifyingFieldsSeen: &vus, IdentifyingMethod: "structure scan",
		IdentifyingAt: time.Now()}
	c := trouve(CheckCard(card, m), "pseudonymised")
	if c.Verdict != VerdictDiffers {
		t.Fatalf("verdict %q, attendu %q", c.Verdict, VerdictDiffers)
	}
	if c.Method != "structure scan" {
		t.Fatalf("la methode doit etre nommee : %q", c.Method)
	}
	// Et l'inverse se confirme.
	aucun := false
	m.IdentifyingFieldsSeen = &aucun
	if c := trouve(CheckCard(card, m), "pseudonymised"); c.Verdict != VerdictAgrees {
		t.Fatalf("verdict %q, attendu %q", c.Verdict, VerdictAgrees)
	}
}

// Un ensemble qui differe dit dans quel sens : une modalite arrivee et une
// modalite partie n'appellent pas la meme conversation.
func TestUnEnsembleQuiDiffereDitDansQuelSens(t *testing.T) {
	card := &Card{Claims: Claims{Modalities: []string{"DICOM", "OCT"}}}
	c := trouve(CheckCard(card, Measured{Modalities: []string{"DICOM", "ANTERION"}, Method: "path scan"}), "modalities")
	if c.Verdict != VerdictDiffers {
		t.Fatalf("verdict %q, attendu %q", c.Verdict, VerdictDiffers)
	}
	if !contient(c.Note, "anterion") || !contient(c.Note, "oct") {
		t.Fatalf("la note doit nommer les deux sens : %q", c.Note)
	}
}

// La prose n'est pas verifiable, et le dire est un verdict.
func TestLaProseEstDeclareeNonVerifiable(t *testing.T) {
	card := &Card{Purpose: "mesurer la progression du glaucome"}
	c := trouve(CheckCard(card, Measured{}), "purpose")
	if c.Verdict != VerdictNotCheckable {
		t.Fatalf("verdict %q, attendu %q", c.Verdict, VerdictNotCheckable)
	}
	if c.Declared == "" {
		t.Fatal("la declaration doit rester visible")
	}
	// Non declaree, c'est "undeclared" et non "not_checkable" : un champ vide
	// et un champ qu'on ne peut pas verifier sont deux etats differents.
	if c := trouve(CheckCard(&Card{}, Measured{}), "purpose"); c.Verdict != VerdictUndeclared {
		t.Fatalf("verdict %q, attendu %q", c.Verdict, VerdictUndeclared)
	}
}

// Declarer zero n'est pas ne rien declarer.
func TestDeclarerZeroNEstPasNeRienDeclarer(t *testing.T) {
	c := trouve(CheckCard(&Card{Claims: Claims{Subjects: entier(0)}}, Measured{Subjects: 32}), "subjects")
	if c.Verdict != VerdictDiffers {
		t.Fatalf("verdict %q, attendu %q : zero declare est une erreur a montrer", c.Verdict, VerdictDiffers)
	}
}

func contient(texte, motif string) bool {
	for i := 0; i+len(motif) <= len(texte); i++ {
		if equalFold(texte[i:i+len(motif)], motif) {
			return true
		}
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		x, y := a[i], b[i]
		if 'A' <= x && x <= 'Z' {
			x += 'a' - 'A'
		}
		if 'A' <= y && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y {
			return false
		}
	}
	return true
}
