package handlers

import (
	"strings"
	"testing"

	jobdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/job"
)

// Rejouer, c'est relancer l'image qui a tourne - pas celle que l'etiquette
// designe aujourd'hui. La reconstruction nocturne fait bouger les etiquettes :
// sans l'empreinte, « rejouer » relancerait un autre logiciel sous le meme nom.
func TestLeRejeuViseLEmpreinteEtNonLEtiquette(t *testing.T) {
	cas := []struct{ image, digest, attendu string }{
		{"harbor.lan/noryx/analyse:0.3.1", "sha256:abc", "harbor.lan/noryx/analyse@sha256:abc"},
		{"harbor.lan/noryx/analyse", "sha256:abc", "harbor.lan/noryx/analyse@sha256:abc"},
		// Un registre avec un port n'a pas d'etiquette pour autant.
		{"harbor.lan:5000/noryx/analyse", "sha256:abc", "harbor.lan:5000/noryx/analyse@sha256:abc"},
		// Deja par empreinte : on n'en ajoute pas une seconde.
		{"harbor.lan/noryx/analyse@sha256:zzz", "sha256:abc", "harbor.lan/noryx/analyse@sha256:zzz"},
		// Pas d'empreinte enregistree : on garde ce qu'on a.
		{"harbor.lan/noryx/analyse:0.3.1", "", "harbor.lan/noryx/analyse:0.3.1"},
	}
	for _, c := range cas {
		if got := imageAtDigest(c.image, c.digest); got != c.attendu {
			t.Errorf("imageAtDigest(%q,%q) = %q, attendu %q", c.image, c.digest, got, c.attendu)
		}
	}
}

// Le nom reste reconnaissable sans enfler a chaque rejeu.
func TestLeNomNEnflePasAChaqueRejeu(t *testing.T) {
	if got := rerunName("analyse"); got != "analyse (rejeu)" {
		t.Fatalf("%q", got)
	}
	if got := rerunName("analyse (rejeu)"); got != "analyse (rejeu)" {
		t.Fatalf("le suffixe s'empile : %q", got)
	}
}

// Un rejeu identique ne signale rien : une liste de divergences qui crie au
// loup a chaque fois ne sera plus lue quand elle aura raison.
func TestUnRejeuIdentiqueNeSignaleRien(t *testing.T) {
	original := jobdomain.Job{
		Datasets:     []jobdomain.Dataset{{ID: "d1", Name: "cohorte"}},
		Repositories: []jobdomain.Repository{{Name: "analyse", Commit: "aaaaaaaabbbb"}},
	}
	got := compareProvenance(original, original.Datasets, original.Repositories)
	if len(got) != 0 {
		t.Fatalf("divergences inventees : %v", got)
	}
}

// Et un rejeu qui ne peut pas etre identique le dit, en nommant quoi.
func TestLesDivergencesSontNommees(t *testing.T) {
	original := jobdomain.Job{
		Datasets:     []jobdomain.Dataset{{ID: "d1", Name: "cohorte"}},
		Repositories: []jobdomain.Repository{{Name: "analyse", Commit: "aaaaaaaa1111"}},
	}
	got := compareProvenance(original,
		[]jobdomain.Dataset{{ID: "d2", Name: "autre"}},
		[]jobdomain.Repository{{Name: "analyse", Commit: "bbbbbbbb2222"}},
	)
	joint := strings.Join(got, " | ")
	for _, attendu := range []string{"cohorte", "autre", "analyse", "aaaaaaaa", "bbbbbbbb"} {
		if !strings.Contains(joint, attendu) {
			t.Errorf("la divergence ne nomme pas %q : %s", attendu, joint)
		}
	}
}

// Un commit inconnu des deux cotes ne doit pas se lire comme « le code est le
// meme » : on ne sait pas, et le dire est la seule reponse honnete.
func TestUnCommitInconnuSeDitInconnu(t *testing.T) {
	original := jobdomain.Job{Repositories: []jobdomain.Repository{{Name: "analyse", Commit: ""}}}
	got := compareProvenance(original, nil, []jobdomain.Repository{{Name: "analyse", Commit: ""}})
	if len(got) != 1 || !strings.Contains(got[0], "unknown") {
		t.Fatalf("%v", got)
	}
}
