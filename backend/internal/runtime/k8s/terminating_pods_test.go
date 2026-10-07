package k8s

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newTestRuntime points a Runtime at a fake API server. Same package, so the
// fields are reachable without a constructor that only tests would use.
func newTestRuntime(apiURL string) *Runtime {
	return &Runtime{
		httpClient:        &http.Client{Timeout: 5 * time.Second},
		apiURL:            apiURL,
		token:             "jeton-de-test",
		controlNamespace:  "noryx",
		workloadNamespace: "noryx-loads",
	}
}

// Un pod en cours de suppression n'est pas un pod.
//
// La suppression est asynchrone chez Kubernetes : le pod garde ses etiquettes
// pendant qu'il se vide, donc une charge supprimee il y a une seconde repond
// encore a un listing. La lire comme presente remettait un workspace sur
// l'ecran qu'il venait de quitter, et - parce que le reconciliateur garantit le
// projet derriere toute charge qu'il voit - ressuscitait des projets supprimes
// en "Recovered Project <id>". Un par nuit pendant une semaine sur EMSE.
func TestUnPodEnSuppressionEstIgnore(t *testing.T) {
	const corps = `{"items":[
	  {"metadata":{"name":"wks-vivant","labels":{"noryx.io/workspace-id":"vivant","noryx.io/project-id":"p1"}},
	   "spec":{"containers":[{"image":"i"}]},"status":{"phase":"Running"}},
	  {"metadata":{"name":"wks-partant","deletionTimestamp":"2026-10-07T02:03:00Z",
	   "labels":{"noryx.io/workspace-id":"partant","noryx.io/project-id":"p2"}},
	   "spec":{"containers":[{"image":"i"}]},"status":{"phase":"Running"}}
	]}`
	serveur := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/pods") {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(corps))
	}))
	defer serveur.Close()

	items, err := newTestRuntime(serveur.URL).ListWorkspaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("%d workspaces renvoyes, attendu 1 : %+v", len(items), items)
	}
	if items[0].WorkspaceID != "vivant" {
		t.Fatalf("workspace renvoye = %q, attendu vivant", items[0].WorkspaceID)
	}
}

// Meme regle pour les builds : un job qui se vide garde ses etiquettes.
func TestUnJobEnSuppressionEstIgnore(t *testing.T) {
	const corps = `{"items":[
	  {"metadata":{"name":"b-vivant","labels":{"noryx.io/build-id":"vivant","noryx.io/project-id":"p1"}},
	   "spec":{"template":{"spec":{}}},"status":{"active":1}},
	  {"metadata":{"name":"b-partant","deletionTimestamp":"2026-10-07T02:03:00Z",
	   "labels":{"noryx.io/build-id":"partant","noryx.io/project-id":"p2"}},
	   "spec":{"template":{"spec":{}}},"status":{"succeeded":1}}
	]}`
	serveur := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(corps))
	}))
	defer serveur.Close()

	items, err := newTestRuntime(serveur.URL).ListBuilds()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].BuildID != "vivant" {
		t.Fatalf("builds renvoyes = %+v, attendu le seul vivant", items)
	}
}
