package handlers

import (
	"testing"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

// Une version lue sur l'image, et rien d'invente.
func TestLaVersionSeLitSurLEtiquetteDeLImage(t *testing.T) {
	cas := map[string]string{
		"harbor.emse.local/noryx-ee/keycloak:26.6.2":             "26.6.2",
		"harbor.emse.local/noryx-ce/postgres:16.15":              "16.15",
		"harbor.lan/noryx-environments/noryx-rstudio:0.1.1":      "0.1.1",
		"harbor.emse.local/x/minio:RELEASE.2025-10-15T17-29-55Z": "RELEASE.2025-10-15T17-29-55Z",
		// Une empreinte n'est pas une version : un evaluateur veut quelque
		// chose a comparer avec un avis de securite.
		"harbor.lan/x/y@sha256:abc": "",
		// "latest" ne nomme pas une version, il nomme une cible mouvante.
		"harbor.lan/x/y:latest": "",
		// Un port dans le registre ne doit pas passer pour une etiquette.
		"registre.local:5000/x/y": "",
		"":                        "",
	}
	for image, attendu := range cas {
		if got := versionFromImage(image); got != attendu {
			t.Errorf("%q -> %q, attendu %q", image, got, attendu)
		}
	}
}

// Le Runner complet est integre pour n'avoir a ecrire que les deux methodes
// qui comptent ici ; un appel a une autre paniquerait, ce qui est la bonne
// facon d'echouer dans un test qui n'en attend aucun.
type inspecteurFactice struct {
	noryxruntime.Runner
	version      string
	deploiements []noryxruntime.DeploymentStatus
}

func (i inspecteurFactice) ListDeployments() ([]noryxruntime.DeploymentStatus, error) {
	return i.deploiements, nil
}
func (i inspecteurFactice) ClusterVersion() (string, error) { return i.version, nil }
func (i inspecteurFactice) ListServices() ([]noryxruntime.ServiceStatus, error) {
	return nil, nil
}

// Le defaut que le rapport d'un hopital a releve le 27/11/2026 : les dix
// briques d'infrastructure sortaient sans version, ce qui empechait de
// verifier leurs vulnerabilites.
func TestLesBriquesSortentAvecLeurVersionDeployee(t *testing.T) {
	h := Handlers{
		runtime: inspecteurFactice{
			version: "v1.30.2+k3s2",
			deploiements: []noryxruntime.DeploymentStatus{
				{Name: "keycloak", Image: "r/keycloak:26.6.2"},
				{Name: "postgres", Image: "r/postgres:16.15"},
				{Name: "minio", Image: "r/minio:RELEASE.2025-10-15"},
			},
		},
		workspaceJupyterImage: "r/noryx-jupyter:0.1.1",
		workspaceVSCodeImage:  "r/noryx-vscode:0.1.5",
		workspaceRStudioImage: "r/noryx-rstudio:0.1.1",
	}

	document, err := h.deployedInventory()
	if err != nil {
		t.Fatal(err)
	}
	trouve := map[string]string{}
	for _, item := range document.Items {
		if item.Component == "infrastructure" {
			trouve[item.Name] = item.Version
		}
	}
	for nom, attendu := range map[string]string{
		"Keycloak":          "26.6.2",
		"PostgreSQL":        "16.15",
		"MinIO":             "RELEASE.2025-10-15",
		"k3s":               "v1.30.2+k3s2",
		"JupyterLab":        "0.1.1",
		"openvscode-server": "0.1.5",
	} {
		if trouve[nom] != attendu {
			t.Errorf("%s = %q, attendu %q", nom, trouve[nom], attendu)
		}
	}
	// Les deux absents que le rapport nommait.
	if _, ok := trouve["RStudio"]; !ok {
		t.Error("RStudio absent de l'inventaire")
	}
}

// Sans runtime interrogeable, on laisse les blancs : inventer une version
// repondrait a la question d'un evaluateur par une supposition.
func TestSansRuntimeLesVersionsRestentVides(t *testing.T) {
	document, err := Handlers{}.deployedInventory()
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range document.Items {
		if item.Component == "infrastructure" && item.Version != "" && item.Origin == "deployed" {
			t.Errorf("%s a recu une version %q sans runtime", item.Name, item.Version)
		}
	}
}
