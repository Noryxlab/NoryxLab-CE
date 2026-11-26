package objectstore

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/minio/minio-go/v7"
)

// Le contrat, verifie sur l'implementation en memoire. Une implementation qui
// ne le tient pas fait passer des tests de handler qui echoueront en vrai, ce
// qui est pire que pas de test du tout.
var _ Store = (*Memory)(nil)
var _ Store = (*S3)(nil)

func seeded(t *testing.T) *Memory {
	t.Helper()
	store := NewMemory("cohorte")
	store.Seed("cohorte", "SELENA-01-003/visite1/oct.e2e", []byte("octets"))
	store.Seed("cohorte", "SELENA-01-003/visite2/oct.e2e", []byte("autres"))
	store.Seed("cohorte", "lisez-moi.txt", []byte("note"))
	return store
}

func TestCeQuiNExistePasEstErrNotFoundEtPasUneErreurDeFournisseur(t *testing.T) {
	store := seeded(t)
	if _, err := store.Stat(context.Background(), "cohorte", "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Stat = %v, attendu ErrNotFound", err)
	}
	if _, err := store.Get(context.Background(), "cohorte", "absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get = %v, attendu ErrNotFound", err)
	}
	// Supprimer ce qui n'est pas la n'est pas une erreur : l'appelant voulait
	// que ce soit parti, et c'est parti.
	if err := store.Remove(context.Background(), "cohorte", "absent"); err != nil {
		t.Fatalf("Remove = %v, attendu nil", err)
	}
}

func TestUnListageNonRecursifSArreteAuDelimiteur(t *testing.T) {
	store := seeded(t)
	paths := []string{}
	err := store.List(context.Background(), "cohorte", ListOptions{}, func(item Object) error {
		paths = append(paths, item.Path)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	// Un dossier, une fois, plus le fichier a la racine - la forme que S3
	// rend et sur laquelle l'explorateur de fichiers compte.
	want := []string{"SELENA-01-003/", "lisez-moi.txt"}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Fatalf("listage = %v, attendu %v", paths, want)
	}
}

func TestUnListageRecursifRendLesObjetsEtLeursEmpreintes(t *testing.T) {
	store := seeded(t)
	seen := map[string]Object{}
	if err := store.List(context.Background(), "cohorte", ListOptions{Recursive: true}, func(item Object) error {
		seen[item.Path] = item
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(seen) != 3 {
		t.Fatalf("%d objets, attendu 3", len(seen))
	}
	first := seen["SELENA-01-003/visite1/oct.e2e"]
	if first.Size != 6 || first.ETag == "" {
		t.Fatalf("taille et empreinte attendues, obtenu %+v", first)
	}
	// Deux contenus differents, deux empreintes differentes : c'est ce dont
	// le croisement d'empreintes depend.
	if first.ETag == seen["SELENA-01-003/visite2/oct.e2e"].ETag {
		t.Error("deux contenus differents partagent une empreinte")
	}
}

// Le rappel permet de s'arreter : un dataset de cent mille objets ne se
// charge pas en memoire parce qu'un appelant cherchait le premier.
func TestUnAppelantPeutSArreter(t *testing.T) {
	store := seeded(t)
	stop := errors.New("assez")
	count := 0
	err := store.List(context.Background(), "cohorte", ListOptions{Recursive: true}, func(Object) error {
		count++
		return stop
	})
	if !errors.Is(err, stop) {
		t.Fatalf("err = %v, attendu l'erreur de l'appelant", err)
	}
	if count != 1 {
		t.Fatalf("%d visites apres un arret immediat", count)
	}
}

func TestUneTailleQuiMentEstRefusee(t *testing.T) {
	store := NewMemory("ouvert")
	_, err := store.Put(context.Background(), "ouvert", "k", strings.NewReader("quatre"), 99, "text/plain")
	if err == nil {
		t.Fatal("une taille declaree fausse doit etre refusee : contre un vrai magasin, c'est un objet tronque que personne ne voit")
	}
}

func TestEcrireRelireCopier(t *testing.T) {
	store := NewMemory("source", "destination")
	body := "contenu"
	stored, err := store.Put(context.Background(), "source", "a.txt", strings.NewReader(body), int64(len(body)), "text/plain")
	if err != nil {
		t.Fatal(err)
	}
	// Ce que le stockage a accepte, pas ce que l'appelant annoncait : c'est
	// cette valeur que la ligne d'audit d'un dataset reglemente enregistre.
	if stored.Size != int64(len(body)) || stored.ContentType != "text/plain" {
		t.Fatalf("Put rend %+v", stored)
	}
	if err := store.Copy(context.Background(), "source", "a.txt", "destination", "b.txt"); err != nil {
		t.Fatal(err)
	}
	reader, err := store.Get(context.Background(), "destination", "b.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	content, _ := io.ReadAll(reader)
	if string(content) != body {
		t.Fatalf("copie = %q, attendu %q", content, body)
	}
}

// Une absence et une illisibilite ne sont pas la meme chose.
//
// La question vient du renommage de dataset, ou elle decide si l'on ecrase :
// lire n'importe quelle erreur comme « absent » laisse un renommage recouvrir
// un fichier dont l'existence n'a pas pu etre verifiee, ce qui sur une donnee
// reglementee est une suppression sans trace de ce qui a ete perdu. Elle est
// tranchee ici, une fois, pour toutes les implementations.
func TestUneErreurDeLectureNEstPasUneAbsence(t *testing.T) {
	absent := []minio.ErrorResponse{
		{Code: "NoSuchKey", StatusCode: 404},
		{Code: "NoSuchBucket", StatusCode: 404},
		// Certains magasins repondent 404 sans code : la premiere version de
		// ce test ne verifiait qu'une des deux formes, et chaque site
		// d'appel ne verifiait que l'autre.
		{Code: "", StatusCode: 404},
	}
	for _, response := range absent {
		if !errors.Is(translateError(response), ErrNotFound) {
			t.Errorf("%+v n'a pas ete reconnu comme une absence", response)
		}
	}
	for _, response := range []minio.ErrorResponse{
		{Code: "AccessDenied", StatusCode: 403},
		{Code: "InternalError", StatusCode: 500},
		{Code: "SlowDown", StatusCode: 503},
	} {
		if errors.Is(translateError(response), ErrNotFound) {
			t.Errorf("%s a ete lu comme une absence ; un renommage ecraserait", response.Code)
		}
	}
	if translateError(nil) != nil {
		t.Error("une absence d'erreur est devenue une erreur")
	}
}

// Une plage, parce qu'un navigateur en demande une : servir une video depuis
// un dataset, c'est repondre a un Range, et un lecteur qui ne sait partir que
// de zero transforme chaque deplacement en re-telechargement complet.
func TestUneLecturePartielleRendExactementLaPlageDemandee(t *testing.T) {
	store := NewMemory("media")
	store.Seed("media", "film.bin", []byte("0123456789"))

	cases := []struct {
		offset, length int64
		want           string
	}{
		{0, 4, "0123"},
		{4, 3, "456"},
		// Longueur negative : jusqu'a la fin.
		{7, -1, "789"},
		// Une longueur qui deborde s'arrete a la fin plutot que d'echouer,
		// comme le fait un serveur de fichiers.
		{8, 100, "89"},
	}
	for _, item := range cases {
		reader, err := store.GetRange(context.Background(), "media", "film.bin", item.offset, item.length)
		if err != nil {
			t.Fatalf("offset %d longueur %d: %v", item.offset, item.length, err)
		}
		got, _ := io.ReadAll(reader)
		reader.Close()
		if string(got) != item.want {
			t.Errorf("offset %d longueur %d = %q, attendu %q", item.offset, item.length, got, item.want)
		}
	}

	if _, err := store.GetRange(context.Background(), "media", "absent", 0, 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("une plage sur un objet absent doit etre ErrNotFound, obtenu %v", err)
	}
}
