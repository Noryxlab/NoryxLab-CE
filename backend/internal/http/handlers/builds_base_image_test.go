package handlers

import (
	"strings"
	"testing"
)

// Le modele vierge disait FROM noryx-python:latest : un nom sans registre,
// que kaniko resout sur Docker Hub, ou il n'existe pas. Le 27/10/2026 sur
// EMSE : "authentication required" contre index.docker.io, pour une image
// que la plateforme fait tourner elle-meme.
func TestUnFromSansRegistreDesigneLImageDeLaPlateforme(t *testing.T) {
	h := Handlers{
		workspaceJupyterImage: "harbor.emse.local/noryx-environments/noryx-jupyter:0.1.1",
		workspaceVSCodeImage:  "harbor.emse.local/noryx-environments/noryx-vscode:0.1.4",
	}
	entree := "# Noryx environment\nFROM noryx-python:latest\n\nRUN pip install pandas\n"
	sortie := h.qualifyDockerfileBase(entree)
	if !strings.Contains(sortie, "FROM harbor.emse.local/noryx-environments/noryx-jupyter:0.1.1\n") {
		t.Fatalf("l'ancien nom noryx-python doit designer l'image Jupyter configuree :\n%s", sortie)
	}
	if !strings.Contains(sortie, "RUN pip install pandas") {
		t.Fatal("le reste du Dockerfile doit rester tel quel")
	}

	// Un tag choisi est garde, sur le depot que la plateforme connait ; les
	// options de FROM et l'alias de stage aussi.
	sortie = h.qualifyDockerfileBase("FROM --platform=linux/amd64 noryx-vscode:0.1.2 AS base\n")
	if sortie != "FROM --platform=linux/amd64 harbor.emse.local/noryx-environments/noryx-vscode:0.1.2 AS base\n" {
		t.Fatalf("tag et options doivent survivre :\n%s", sortie)
	}

	// Une reference qui nomme deja un registre, ou un nom inconnu, ne bouge
	// pas : la personne peut vouloir Docker Hub.
	for _, intact := range []string{"FROM harbor.emse.local/noryx-environments/noryx-vscode:0.1.4\n", "FROM python:3.12-slim\n", "FROM ${BASE}\n"} {
		if got := h.qualifyDockerfileBase(intact); got != intact {
			t.Fatalf("%q a ete reecrit en %q", intact, got)
		}
	}
}

// "failed" seul n'est pas un message : la derniere ligne d'erreur du build
// est gardee, sans les codes couleur du terminal.
func TestLaRaisonDUnEchecEstLaDerniereLigneDErreur(t *testing.T) {
	logs := "\x1b[36mINFO\x1b[0m[0000] Retrieving image manifest noryx-python:latest\nerror building image: unable to complete operation after 0 attempts, last error: GET https://index.docker.io/v2/library/noryx-python/manifests/latest: UNAUTHORIZED: authentication required\n\n"
	raison := failureReasonFrom(logs)
	if !strings.HasPrefix(raison, "error building image") || strings.Contains(raison, "\x1b") {
		t.Fatalf("raison = %q", raison)
	}
	if got := failureReasonFrom("step 1\nstep 2\n"); got != "step 2" {
		t.Fatalf("sans ligne d'erreur, la derniere ligne : %q", got)
	}
}
