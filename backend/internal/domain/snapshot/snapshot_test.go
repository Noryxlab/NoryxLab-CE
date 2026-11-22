package snapshot

import (
	"sort"
	"testing"
	"time"
)

func entry(path, etag string, size int64) Entry {
	return Entry{Path: path, ETag: etag, Size: size, LastModified: time.Unix(0, 0)}
}

func kinds(changes []Change) map[string]string {
	out := map[string]string{}
	for _, c := range changes {
		out[c.Path] = c.Kind
	}
	return out
}

// Un dataset qui n'a pas bouge ne produit aucune difference. Une comparaison
// qui signale du bruit a chaque passage ne sera plus lue le jour ou elle aura
// raison.
func TestUnDatasetInchangeNeProduitRien(t *testing.T) {
	avant := []Entry{entry("a/x.e2e", "aaa", 10), entry("a/y.e2e", "bbb", 20)}
	if got := Compare(avant, avant); len(got) != 0 {
		t.Fatalf("differences inventees : %v", got)
	}
}

// Les trois formes de changement sont nommees, pas comptees.
func TestLesTroisFormesDeChangementSontNommees(t *testing.T) {
	avant := []Entry{entry("garde.e2e", "aaa", 10), entry("parti.e2e", "bbb", 20), entry("modifie.e2e", "ccc", 30)}
	maintenant := []Entry{entry("garde.e2e", "aaa", 10), entry("modifie.e2e", "ddd", 30), entry("nouveau.e2e", "eee", 40)}

	got := kinds(Compare(avant, maintenant))
	attendu := map[string]string{"parti.e2e": Removed, "modifie.e2e": Modified, "nouveau.e2e": Added}
	if len(got) != len(attendu) {
		t.Fatalf("%v", got)
	}
	for path, kind := range attendu {
		if got[path] != kind {
			t.Errorf("%s : %q au lieu de %q", path, got[path], kind)
		}
	}
}

// Une taille identique avec une empreinte differente reste un changement.
// L'ETag n'est pas une somme de controle portable : il peut differer pour les
// memes octets, donc il sait dire « ca a change » et jamais « c'est pareil ».
// En cas de doute on signale, parce que la reponse honnete est qu'on ne sait
// pas, pas que rien n'a bouge.
func TestMemeTailleEmpreinteDifferenteEstUnChangement(t *testing.T) {
	got := Compare([]Entry{entry("x.e2e", "aaa", 100)}, []Entry{entry("x.e2e", "zzz", 100)})
	if len(got) != 1 || got[0].Kind != Modified {
		t.Fatalf("%v", got)
	}
	if got[0].Was == nil || got[0].Now == nil {
		t.Fatal("une modification doit porter les deux etats, sinon on ne peut pas l'instruire")
	}
}

// Le cas qui a motive tout ca : un fichier reecrit sous une etude finie.
// Avant, ni l'ontologie ni l'extrait ne gardaient d'empreinte, donc rien dans
// la plateforme ne pouvait le remarquer.
func TestUnFichierReecritEstDetecte(t *testing.T) {
	avant := []Entry{entry("SELENA-01-003/visit1/oct.e2e", "d41d8c", 51200)}
	maintenant := []Entry{entry("SELENA-01-003/visit1/oct.e2e", "9f86d0", 51200)}
	got := Compare(avant, maintenant)
	if len(got) != 1 || got[0].Path != "SELENA-01-003/visit1/oct.e2e" {
		t.Fatalf("%v", got)
	}
}

// Un dataset vide des deux cotes, et le passage de vide a plein : les bornes
// ne doivent ni paniquer ni inventer.
func TestLesBornesSontCalmes(t *testing.T) {
	if got := Compare(nil, nil); len(got) != 0 {
		t.Fatalf("%v", got)
	}
	got := Compare(nil, []Entry{entry("a", "aaa", 1), entry("b", "bbb", 2)})
	paths := []string{}
	for _, c := range got {
		if c.Kind != Added {
			t.Errorf("%s : %q", c.Path, c.Kind)
		}
		paths = append(paths, c.Path)
	}
	sort.Strings(paths)
	if len(paths) != 2 {
		t.Fatalf("%v", paths)
	}
}
