package handlers

import (
	"testing"
)

// Le commit est ce qui distingue « le code d'aujourd'hui » de « le code de ce
// jour-la ». Sans lui, l'enregistrement repond a la question en pointant une
// reference qui a bouge.
func TestLeCommitEstEnregistreAvecLaReference(t *testing.T) {
	repos := jobRepositoriesFrom(
		[]workspaceAttachedRepo{{Name: "analyse", URL: "https://forge/analyse.git", DefaultRef: "main"}},
		func(url, ref string) string { return "9f1c2ab0000000000000000000000000000000aa" },
	)
	if len(repos) != 1 {
		t.Fatalf("%d depots enregistres", len(repos))
	}
	if repos[0].Commit != "9f1c2ab0000000000000000000000000000000aa" {
		t.Fatalf("commit = %q", repos[0].Commit)
	}
	if repos[0].Ref != "main" {
		t.Fatalf("la reference demandee n'est pas gardee a cote : %q", repos[0].Ref)
	}
}

// Une forge muette ne doit pas empecher un calcul de tourner. Le commit vide
// se lit « inconnu », ce qui est une reponse ; refuser le lancement n'en est
// pas une.
func TestUneForgeMuetteNEmpechePasLeRun(t *testing.T) {
	repos := jobRepositoriesFrom(
		[]workspaceAttachedRepo{{Name: "analyse", URL: "https://forge/analyse.git", DefaultRef: "main"}},
		func(url, ref string) string { return "" },
	)
	if len(repos) != 1 || repos[0].Commit != "" {
		t.Fatalf("%+v", repos)
	}
	if repos[0].URL == "" {
		t.Error("l'adresse du depot est perdue, donc on ne sait meme plus quoi aller chercher")
	}
}

// Un projet sans depot produit une liste vide, pas nil : une colonne JSONB qui
// recoit `null` casse la lecture suivante, et une provenance qu'on ne sait plus
// relire ne vaut pas mieux que pas de provenance.
func TestSansDepotLaListeEstVideEtNonNulle(t *testing.T) {
	repos := jobRepositoriesFrom(nil, resolveCommitSHA)
	if repos == nil {
		t.Fatal("nil au lieu d'une liste vide")
	}
	if len(repos) != 0 {
		t.Fatalf("%d depots inventes", len(repos))
	}
}

// On reutilise le resolveur des constructions d'environnement plutot que d'en
// ecrire un second. Ce test fige ce choix : un resolveur qui appellerait le
// binaire git echouerait a tous les coups, le backend etant distroless.
func TestLeResolveurEstCeluiDesConstructions(t *testing.T) {
	if got := resolveCommitSHA("https://127.0.0.1:1/absent.git", "main"); got != "" {
		t.Fatalf("commit invente : %q", got)
	}
	repos := jobRepositoriesFrom(
		[]workspaceAttachedRepo{{Name: "x", URL: "git@forge:x.git", DefaultRef: "main"}},
		resolveCommitSHA,
	)
	if repos[0].Commit != "" {
		t.Fatalf("un distant ssh ne peut pas etre resolu sans cle : %q", repos[0].Commit)
	}
}
