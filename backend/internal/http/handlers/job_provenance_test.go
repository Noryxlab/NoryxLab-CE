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
		func(url, ref, token string) string { return "9f1c2ab0000000000000000000000000000000aa" },
		nil,
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
		func(url, ref, token string) string { return "" },
		nil,
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
	repos := jobRepositoriesFrom(nil, resolveCommitSHAAs, nil)
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
	if got := resolveCommitSHAAs("https://127.0.0.1:1/absent.git", "main", ""); got != "" {
		t.Fatalf("commit invente : %q", got)
	}
	repos := jobRepositoriesFrom(
		[]workspaceAttachedRepo{{Name: "x", URL: "git@forge:x.git", DefaultRef: "main"}},
		resolveCommitSHAAs, nil,
	)
	if repos[0].Commit != "" {
		t.Fatalf("un distant ssh ne peut pas etre resolu sans cle : %q", repos[0].Commit)
	}
}

// Le jeton du depot est bien celui que la plateforme detient, et il n'est
// demande que pour les depots qui en ont un.
//
// Mesure sur le DC le 2026-11-22 : GitHub repond 200 a l'annonce de references
// d'un depot public et 401 « Repository not found » a celle d'un depot prive.
// Tous les depots attaches ici sont prives, donc sans ce jeton le commit
// serait vide a chaque fois - un champ de provenance toujours blanc est pire
// que pas de champ, parce qu'il ressemble a une reponse.
func TestLeJetonDuDepotEstTransmisAuResolveur(t *testing.T) {
	vus := map[string]string{}
	repos := jobRepositoriesFrom(
		[]workspaceAttachedRepo{
			{Name: "prive", URL: "https://forge/prive.git", DefaultRef: "main", AuthSecretName: "forge-token"},
			{Name: "public", URL: "https://forge/public.git", DefaultRef: "main"},
		},
		func(url, ref, token string) string { vus[url] = token; return "abc" },
		func(secretName string) string {
			if secretName == "forge-token" {
				return "t0ken"
			}
			return ""
		},
	)
	if len(repos) != 2 {
		t.Fatalf("%d depots", len(repos))
	}
	if vus["https://forge/prive.git"] != "t0ken" {
		t.Errorf("le jeton n'est pas transmis : %q", vus["https://forge/prive.git"])
	}
	if vus["https://forge/public.git"] != "" {
		t.Errorf("un jeton est invente pour un depot qui n'en a pas : %q", vus["https://forge/public.git"])
	}
}

// La raison d'une lecture seule voyage avec le drapeau.
//
// Le 2026-11-25, Cédric (Essilor) possede un bucket non reglemente, l'attache
// a un projet qui monte aussi de la donnee HDS, et son job echoue en y
// ecrivant : la regle de co-montage (ADR-055) l'avait passe en lecture seule.
// Le lancement l'avait enregistre dans l'audit et l'ecran des jobs n'en disait
// rien - « lecture seule » sans raison envoie le proprietaire d'un dataset
// chercher un droit qu'il detient deja.
func TestLaRaisonDUneLectureSeuleVoyageAvecLeDrapeau(t *testing.T) {
	datasets := jobDatasetsFrom([]workspaceAttachedDataset{
		{ID: "a", Name: "hds-cohorte", Classification: "hds", ReadOnly: false},
		{ID: "b", Name: "nonhds-resultats", Classification: "non-hds", ReadOnly: true, ReadOnlyReason: readOnlyReasonRegulatedCoMount},
		{ID: "c", Name: "lecture-par-droit", Classification: "non-hds", ReadOnly: true},
	})
	if len(datasets) != 3 {
		t.Fatalf("%d datasets enregistres", len(datasets))
	}
	if datasets[1].ReadOnlyReason != "regulated-co-mount" {
		t.Errorf("la raison du co-montage est perdue : %q", datasets[1].ReadOnlyReason)
	}
	// Un montage en lecture seule par le droit de la personne n'a pas de
	// raison a donner : « vous avez Read » n'est pas une surprise.
	if datasets[2].ReadOnlyReason != "" {
		t.Errorf("une lecture seule par droit ne doit porter aucune raison, got %q", datasets[2].ReadOnlyReason)
	}
	if datasets[0].ReadOnly || datasets[0].ReadOnlyReason != "" {
		t.Errorf("le dataset reglemente en ecriture ne doit rien porter : %+v", datasets[0])
	}
}
