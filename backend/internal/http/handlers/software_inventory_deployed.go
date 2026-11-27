package handlers

import (
	"sort"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/inventory"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

// Filling in the versions the inventory could not know when it was generated.
//
// The embedded document is produced at build time from the resolved
// dependency graphs, which is right for Go and npm: those versions are a
// property of the release. It is wrong for the infrastructure, whose versions
// are a property of the *installation* - Keycloak 26.6.2 here, something else
// there - so the file shipped them blank.
//
// Blank is what a security reviewer met. From an actual review of this
// platform, 2026-11-27: "Aucune version n'est indiquée pour ces 10 briques,
// ce qui nous empêche de vérifier leurs vulnérabilités." Ten, and they had
// counted correctly. An inventory that cannot name what it runs does not
// merely omit something: it stops the work that asked for it.
//
// So the versions are read from the cluster at the moment the inventory is
// asked for. What the platform deploys, it can see; what it configures, it
// knows. Nothing is guessed - a brick the installation does not run keeps an
// empty version rather than inheriting a plausible one.
func (h Handlers) deployedInventory() (inventory.Document, error) {
	document, err := inventory.Parse()
	if err != nil {
		return inventory.Document{}, err
	}

	versions := h.deployedVersions()
	for index := range document.Items {
		item := &document.Items[index]
		if item.Component != "infrastructure" && item.Component != "platform" {
			continue
		}
		if strings.TrimSpace(item.Version) != "" {
			continue
		}
		if version, found := versions[strings.ToLower(item.Name)]; found {
			item.Version = version
			item.Origin = "deployed"
		}
	}

	// The two the list was missing, reported by the same review. RStudio is a
	// workspace image this installation offers; Harbor is the registry it
	// pulls from. Both were invisible because neither is a dependency of the
	// build, which is exactly the blind spot a build-time inventory has.
	for _, extra := range h.deployedExtras(versions) {
		document.Items = append(document.Items, extra)
	}
	sort.SliceStable(document.Items, func(i, j int) bool {
		if document.Items[i].Component != document.Items[j].Component {
			return document.Items[i].Component < document.Items[j].Component
		}
		return strings.ToLower(document.Items[i].Name) < strings.ToLower(document.Items[j].Name)
	})
	document.Counts["total"] = len(document.Items)
	return document, nil
}

// deployedVersions maps a brick's inventory name to the version this
// installation runs.
func (h Handlers) deployedVersions() map[string]string {
	out := map[string]string{}
	inspector, ok := h.runtime.(noryxruntime.Inspector)
	if !ok {
		// An installation whose runtime cannot be asked keeps the blanks. The
		// alternative - inventing a version from the image the platform was
		// built against - would answer the reviewer's question with a guess,
		// which is the one thing an inventory must never do.
		return out
	}
	if version, err := inspector.ClusterVersion(); err == nil && version != "" {
		out["k3s"] = version
		out["kubernetes"] = version
	}
	deployments, err := inspector.ListDeployments()
	if err != nil {
		return out
	}
	// A deployment's name is the platform's own, and the inventory's name is
	// the product's: the pairing is written here rather than guessed from a
	// substring, because "postgres" appears inside "postgres-backup" and a
	// wrong version is worse than none.
	named := map[string]string{
		"keycloak":       "keycloak",
		"minio":          "minio",
		"postgres":       "postgresql",
		"noryx-backend":  "noryxlab enterprise edition",
		"noryx-frontend": "nginx",
	}
	for _, deployment := range deployments {
		brick, known := named[strings.ToLower(strings.TrimSpace(deployment.Name))]
		if !known {
			continue
		}
		if version := versionFromImage(deployment.Image); version != "" {
			out[brick] = version
		}
	}
	// The workspace images, which are configuration rather than deployments.
	for brick, image := range map[string]string{
		"jupyterlab":        h.workspaceJupyterImage,
		"openvscode-server": h.workspaceVSCodeImage,
		"rstudio":           h.workspaceRStudioImage,
	} {
		if version := versionFromImage(image); version != "" {
			out[brick] = version
		}
	}
	return out
}

// deployedExtras are the components the build-time inventory cannot see
// because nothing in the build depends on them.
func (h Handlers) deployedExtras(versions map[string]string) []inventory.Item {
	extras := []inventory.Item{}
	if version, found := versions["rstudio"]; found {
		extras = append(extras, inventory.Item{
			Name: "RStudio", Version: version, Licence: "AGPL-3.0",
			Component: "infrastructure", Origin: "deployed",
			Role: "R workspace image", Upstream: "https://posit.co/products/open-source/rstudio-server/",
		})
	}
	if host := registryHost(h.harborURL); host != "" {
		extras = append(extras, inventory.Item{
			Name: "Harbor", Version: "", Licence: "Apache-2.0",
			Component: "infrastructure", Origin: "declared",
			Role: "container registry", Upstream: "https://goharbor.io/",
		})
	}
	return extras
}

// versionFromImage reads the tag of an image reference.
//
// A digest yields nothing rather than "sha256:…": a reviewer asking for a
// version wants one they can compare against an advisory, and a digest is not
// that. A tag that is "latest" is equally useless and is also refused - it
// names no version, it names a moving target.
func versionFromImage(image string) string {
	image = strings.TrimSpace(image)
	if image == "" {
		return ""
	}
	if at := strings.Index(image, "@"); at >= 0 {
		image = image[:at]
	}
	colon := strings.LastIndex(image, ":")
	if colon < 0 || colon < strings.LastIndex(image, "/") {
		return ""
	}
	tag := strings.TrimSpace(image[colon+1:])
	if tag == "" || strings.EqualFold(tag, "latest") {
		return ""
	}
	return tag
}
