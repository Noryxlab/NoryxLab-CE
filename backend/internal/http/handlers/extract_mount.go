package handlers

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"log"
	"strings"

	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
)

// Mounting an extract: a tree of links, not a copy.
//
// The point of an extract is to work on a subset organised the way the study
// thinks - subject, visit, modality - while the bytes stay exactly where they
// are. So the workspace gets a directory of symlinks pointing into the dataset
// mount, and the source bucket is never written to, never copied, never
// touched beyond the listing that built the ontology in the first place.
//
// The file list travels in the bootstrap secret, gzipped: a Kubernetes secret
// is capped at a megabyte, so a very large extract is refused out loud in the
// bootstrap log rather than mounted as a partial tree that would silently be a
// different study.

const (
	// Well under the 1 MiB secret ceiling, leaving room for the script itself.
	extractManifestMaxBytes = 700 * 1024
	workspaceExtractsPath   = "extracts"
)

type extractMountEntry struct {
	ExtractName string
	DatasetDir  string
	SubjectID   string
	Visit       string
	Modality    string
	Path        string
	// Leaf is where the file sits under the modality, and it is not its name.
	//
	// The tree used to place each file under its basename, which loses every
	// file whose name repeats. DICOM slices are named by their number -
	// 00000121 appears in every series - so on PREMYOM1000's ANTERION modality
	// 21 730 objects landed as 18 825 files and nobody was told: a study
	// silently 13% smaller, which is the worst way to be wrong about data.
	//
	// Keeping the path below the modality fixes it and gives back what the
	// flattening also threw away: the series structure DICOM carries in its
	// directories.
	Leaf string
}

// extractLeaf is the part of a path that belongs under the modality directory.
//
// Found by the modality's own segment, which this layout writes either as
// "ANTERION" or as "modality_ANTERION" - the second in the older tree kept
// under old/. Anything unrecognised keeps its whole path, which is longer than
// it needs to be and never wrong.
func extractLeaf(path, modality string) string {
	segments := strings.Split(strings.Trim(path, "/"), "/")
	wanted := strings.ToLower(strings.TrimSpace(modality))
	if wanted != "" {
		for index := len(segments) - 1; index >= 0; index-- {
			segment := strings.ToLower(segments[index])
			if segment == wanted || segment == "modality_"+wanted {
				if index+1 < len(segments) {
					return strings.Join(segments[index+1:], "/")
				}
				break
			}
		}
	}
	return strings.Join(segments, "/")
}

// encodeExtractManifest packs the entries as gzipped TSV, base64 for transport
// through a secret's string field. It reports whether the result fits.
func encodeExtractManifest(entries []extractMountEntry) (string, bool) {
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
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			entry.ExtractName, entry.DatasetDir, entry.SubjectID, entry.Visit, entry.Modality, entry.Path, entry.Leaf)
	}
	if err := writer.Close(); err != nil {
		return "", false
	}
	encoded := base64.StdEncoding.EncodeToString(raw.Bytes())
	if len(encoded) > extractManifestMaxBytes {
		return "", false
	}
	return encoded, true
}

// extractBootstrapLines rebuilds the tree at every start: the links are cheap,
// and a workspace whose extract changed must not keep yesterday's shape.
func extractBootstrapLines(projectMountPath string, hasManifest bool, refusedCount int) []string {
	root := projectMountPath + "/" + workspaceExtractsPath
	if refusedCount > 0 {
		return []string{
			fmt.Sprintf("echo '[bootstrap] %d extract file(s) not mounted: the selection is too large to ship in one manifest'", refusedCount),
			fmt.Sprintf("echo '[bootstrap] the extract is intact on the platform; open it there to see what it holds'"),
		}
	}
	if !hasManifest {
		return nil
	}
	return []string{
		"if [ -f /var/run/noryx/bootstrap/extracts.b64 ]; then",
		"  echo '[bootstrap] building extract links'",
		fmt.Sprintf("  rm -rf %s && mkdir -p %s", shellQuote(root), shellQuote(root)),
		// Links, never copies: the data stays in the dataset mount, which is
		// mounted read-only, and the extract is a second way of looking at it.
		"  base64 -d /var/run/noryx/bootstrap/extracts.b64 2>/dev/null | gunzip 2>/dev/null | while IFS='\t' read -r extract dataset subject visit modality path feuille; do",
		fmt.Sprintf("    target=/datasets/\"$dataset\"/\"$path\"; dir=%s/\"$extract\"/\"$subject\"/\"$visit\"/\"$modality\"", shellQuote(root)),
		"    mkdir -p \"$dir\"/\"$(dirname \"$feuille\")\" 2>/dev/null || continue",
		"    ln -sfn \"$target\" \"$dir\"/\"$feuille\" 2>/dev/null || true",
		"  done",
		fmt.Sprintf("  echo \"[bootstrap] extract links ready: $(find %s -type l 2>/dev/null | wc -l) file(s)\"", shellQuote(root)),
		"fi",
	}
}

// extractMountEntries resolves the extracts a project's workspace should see.
// An extract whose dataset is not mounted in this workspace is left out: a link
// into a directory that does not exist is a broken file, and a broken file in a
// study directory is worse than an absent one.
func (h Handlers) extractMountEntries(projectID string, attachedDatasets []workspaceAttachedDataset) []extractMountEntry {
	if h.extractStore == nil || strings.TrimSpace(projectID) == "" {
		return nil
	}
	// Lu dans la table de liens, et non dans une colonne de l extrait.
	//
	// Un extrait portait le projet qui le monterait, decide a sa declaration :
	// un seul, pour toujours, et faux des qu on voulait le monter ailleurs.
	// Le rattachement est desormais un lien, comme pour un dataset ou une
	// ontologie, donc plusieurs projets peuvent monter le meme extrait sans
	// qu on le duplique.
	extracts, err := h.projectExtracts(projectID)
	if err != nil {
		log.Printf("workspace started without extract links for project %s: %v", projectID, err)
		return nil
	}
	if len(extracts) == 0 {
		return nil
	}

	mounted := map[string]string{}
	for _, item := range attachedDatasets {
		mounted[strings.ToLower(strings.TrimSpace(item.Name))] = sanitizeWorkspacePathName(item.Name)
	}

	entries := []extractMountEntry{}
	for _, item := range extracts {
		ontology, found, err := h.ontologyStore.GetByID(item.OntologyID)
		if err != nil || !found {
			continue
		}
		directory, ok := mounted[strings.ToLower(strings.TrimSpace(ontology.SourceName))]
		if !ok {
			log.Printf("extract %s not mounted: its dataset %q is not attached to this workspace", item.ID, ontology.SourceName)
			continue
		}
		members, err := h.extractStore.ListMembers(item.ID, 0)
		if err != nil {
			log.Printf("extract %s not mounted: %v", item.ID, err)
			continue
		}
		name := sanitizeWorkspacePathName(item.Name)
		for _, member := range members {
			entries = append(entries, extractMountEntry{
				ExtractName: name,
				DatasetDir:  directory,
				SubjectID:   sanitizeWorkspacePathName(member.SubjectID),
				Visit:       sanitizeWorkspacePathName(member.Visit),
				Modality:    sanitizeWorkspacePathName(member.Modality),
				Path:        member.Path,
				Leaf:        extractLeaf(member.Path, member.Modality),
			})
		}
	}
	return entries
}

// projectExtracts reads what this project has attached.
func (h Handlers) projectExtracts(projectID string) ([]extractdomain.Extract, error) {
	if h.projectResourceStore == nil {
		return nil, nil
	}
	ids, err := h.projectResourceStore.ListProjectExtractIDs(projectID)
	if err != nil {
		return nil, err
	}
	out := make([]extractdomain.Extract, 0, len(ids))
	for _, id := range ids {
		item, found, err := h.extractStore.GetByID(id)
		if err != nil {
			return nil, err
		}
		// Un lien vers un extrait disparu est ignore plutot que fatal : il ne
		// doit pas empecher les autres de se monter.
		if found {
			out = append(out, item)
		}
	}
	return out, nil
}
