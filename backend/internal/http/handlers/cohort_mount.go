package handlers

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"log"
	"strings"
)

// Mounting a cohort: a tree of links, not a copy.
//
// The point of a cohort is to work on a subset organised the way the study
// thinks - subject, visit, modality - while the bytes stay exactly where they
// are. So the workspace gets a directory of symlinks pointing into the dataset
// mount, and the source bucket is never written to, never copied, never
// touched beyond the listing that built the ontology in the first place.
//
// The file list travels in the bootstrap secret, gzipped: a Kubernetes secret
// is capped at a megabyte, so a very large cohort is refused out loud in the
// bootstrap log rather than mounted as a partial tree that would silently be a
// different study.

const (
	// Well under the 1 MiB secret ceiling, leaving room for the script itself.
	cohortManifestMaxBytes = 700 * 1024
	workspaceCohortsPath   = "cohorts"
)

type cohortMountEntry struct {
	CohortName string
	DatasetDir string
	SubjectID  string
	Visit      string
	Modality   string
	Path       string
}

// encodeCohortManifest packs the entries as gzipped TSV, base64 for transport
// through a secret's string field. It reports whether the result fits.
func encodeCohortManifest(entries []cohortMountEntry) (string, bool) {
	if len(entries) == 0 {
		return "", true
	}
	var raw bytes.Buffer
	writer := gzip.NewWriter(&raw)
	for _, entry := range entries {
		// A tab or a newline in a key would split a record and mount a file
		// under the wrong subject. Such a key is skipped, not repaired.
		if strings.ContainsAny(entry.Path, "\t\n") {
			continue
		}
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\n",
			entry.CohortName, entry.DatasetDir, entry.SubjectID, entry.Visit, entry.Modality, entry.Path)
	}
	if err := writer.Close(); err != nil {
		return "", false
	}
	encoded := base64.StdEncoding.EncodeToString(raw.Bytes())
	if len(encoded) > cohortManifestMaxBytes {
		return "", false
	}
	return encoded, true
}

// cohortBootstrapLines rebuilds the tree at every start: the links are cheap,
// and a workspace whose cohort changed must not keep yesterday's shape.
func cohortBootstrapLines(projectMountPath string, hasManifest bool, refusedCount int) []string {
	root := projectMountPath + "/" + workspaceCohortsPath
	if refusedCount > 0 {
		return []string{
			fmt.Sprintf("echo '[bootstrap] %d cohort file(s) not mounted: the selection is too large to ship in one manifest'", refusedCount),
			fmt.Sprintf("echo '[bootstrap] the cohort is intact on the platform; open it there to see what it holds'"),
		}
	}
	if !hasManifest {
		return nil
	}
	return []string{
		"if [ -f /var/run/noryx/bootstrap/cohorts.b64 ]; then",
		"  echo '[bootstrap] building cohort links'",
		fmt.Sprintf("  rm -rf %s && mkdir -p %s", shellQuote(root), shellQuote(root)),
		// Links, never copies: the data stays in the dataset mount, which is
		// mounted read-only, and the cohort is a second way of looking at it.
		"  base64 -d /var/run/noryx/bootstrap/cohorts.b64 2>/dev/null | gunzip 2>/dev/null | while IFS='\t' read -r cohort dataset subject visit modality path; do",
		fmt.Sprintf("    target=/datasets/\"$dataset\"/\"$path\"; dir=%s/\"$cohort\"/\"$subject\"/\"$visit\"/\"$modality\"", shellQuote(root)),
		"    mkdir -p \"$dir\" 2>/dev/null || continue",
		"    ln -sfn \"$target\" \"$dir\"/\"$(basename \"$path\")\" 2>/dev/null || true",
		"  done",
		fmt.Sprintf("  echo \"[bootstrap] cohort links ready: $(find %s -type l 2>/dev/null | wc -l) file(s)\"", shellQuote(root)),
		"fi",
	}
}

// cohortMountEntries resolves the cohorts a project's workspace should see.
// A cohort whose dataset is not mounted in this workspace is left out: a link
// into a directory that does not exist is a broken file, and a broken file in a
// study directory is worse than an absent one.
func (h Handlers) cohortMountEntries(projectID string, attachedDatasets []workspaceAttachedDataset) []cohortMountEntry {
	if h.cohortStore == nil || strings.TrimSpace(projectID) == "" {
		return nil
	}
	cohorts, err := h.cohortStore.ListByProject(projectID)
	if err != nil {
		log.Printf("workspace started without cohort links for project %s: %v", projectID, err)
		return nil
	}
	if len(cohorts) == 0 {
		return nil
	}

	mounted := map[string]string{}
	for _, item := range attachedDatasets {
		mounted[strings.ToLower(strings.TrimSpace(item.Name))] = sanitizeWorkspacePathName(item.Name)
	}

	entries := []cohortMountEntry{}
	for _, item := range cohorts {
		ontology, found, err := h.ontologyStore.GetByID(item.OntologyID)
		if err != nil || !found {
			continue
		}
		directory, ok := mounted[strings.ToLower(strings.TrimSpace(ontology.SourceName))]
		if !ok {
			log.Printf("cohort %s not mounted: its dataset %q is not attached to this workspace", item.ID, ontology.SourceName)
			continue
		}
		members, err := h.cohortStore.ListMembers(item.ID, 0)
		if err != nil {
			log.Printf("cohort %s not mounted: %v", item.ID, err)
			continue
		}
		name := sanitizeWorkspacePathName(item.Name)
		for _, member := range members {
			entries = append(entries, cohortMountEntry{
				CohortName: name,
				DatasetDir: directory,
				SubjectID:  sanitizeWorkspacePathName(member.SubjectID),
				Visit:      sanitizeWorkspacePathName(member.Visit),
				Modality:   sanitizeWorkspacePathName(member.Modality),
				Path:       member.Path,
			})
		}
	}
	return entries
}
