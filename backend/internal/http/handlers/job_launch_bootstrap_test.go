package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// Un job parti comme l'ecran l'envoie passe par le bootstrap.
//
// C'est le maillon que les tests de script ne couvraient pas : le script etait
// juste, et le handler ne l'appelait pas. Le formulaire poste ce que la
// personne tape dans `command`, et le bootstrap ne se construisait que si ce
// champ etait vide - donc jamais, depuis l'ecran. Ce test part du corps HTTP
// que l'UI envoie et regarde ce que le runtime recoit.
func TestUnJobLanceDepuisLEcranPasseParLeBootstrap(t *testing.T) {
	projects := memory.NewProjectStore()
	item := project.NewOwned("owner", "Job project", "")
	if err := projects.Create(item); err != nil {
		t.Fatal(err)
	}
	accessStore := memory.NewAccessStore()
	accessStore.SetRole(item.ID, "member", access.RoleEditor)

	runner := &recordingRuntime{}
	h := Handlers{
		projectStore:         projects,
		accessStore:          accessStore,
		jobStore:             memory.NewJobStore(),
		sessionStore:         memory.NewSessionStore(),
		hardwareTierStore:    memory.NewHardwareTierStore(),
		projectResourceStore: memory.NewProjectResourceStore(),
		repositoryStore:      memory.NewRepositoryStore(),
		datasetStore:         memory.NewDatasetStore(),
		datasourceStore:      memory.NewDatasourceStore(),
		secretStore:          memory.NewSecretStore(),
		auditStore:           memory.NewAuditStore(),
		runtime:              runner,
		workspaceNamespace:   "noryx-loads",
		authMode:             "header",
	}

	// Exactement ce que poste le formulaire des jobs : la commande tapee,
	// decoupee en mots, dans `command`.
	body, err := json.Marshal(map[string]any{
		"projectId": item.ID,
		"image":     "harbor/python:1",
		"command":   []string{"python3", "analyse.py"},
	})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/jobs", bytes.NewReader(body))
	request.Header.Set("X-Noryx-User", "member")
	recorder := httptest.NewRecorder()
	h.CreateJob(recorder, request)
	if recorder.Code >= 300 {
		t.Fatalf("le lancement a echoue: %d %s", recorder.Code, recorder.Body.String())
	}
	if len(runner.jobs) != 1 {
		t.Fatalf("attendu un job, recu %d", len(runner.jobs))
	}

	spec := runner.jobs[0]
	if strings.Join(spec.Command, " ") != "/bin/sh -lc" {
		t.Fatalf("le conteneur doit lancer le script, pas la commande brute: %v", spec.Command)
	}
	if len(spec.Args) != 1 {
		t.Fatalf("attendu un seul argument, le script: %v", spec.Args)
	}
	script := spec.Args[0]
	for _, attendu := range []string{
		"[bootstrap]",
		"/extracts",
		"exec 'python3' 'analyse.py'",
	} {
		if !strings.Contains(script, attendu) {
			t.Fatalf("le script du job doit contenir %q:\n%s", attendu, script)
		}
	}
	// Et la commande de la personne reste lisible sur la fiche du job.
	var reponse map[string]any
	if err := json.Unmarshal(recorder.Body.Bytes(), &reponse); err != nil {
		t.Fatal(err)
	}
	if commande, _ := reponse["command"].([]any); len(commande) != 2 {
		t.Fatalf("la fiche doit garder la commande tapee, recu %v", reponse["command"])
	}
}
