package handlers

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"log"
	"sort"
	"strings"

	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
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
	// A sibling of /datasets and /repos, not a directory inside the project's
	// own volume.
	//
	// It lived at /mnt/extracts, which put a platform-built tree inside the
	// space the project writes to - and the tree is rebuilt with `rm -rf` at
	// every start, so a person who had made their own /mnt/extracts would have
	// lost it. It also read wrong: /datasets holds the buckets, /repos the
	// code, /mnt the project's own work, and an extract is a read-only view of
	// a bucket. It belongs beside the bucket.
	workspaceExtractsPath = "/extracts"
)

// extractManifestFields names the manifest's columns, in order, and is the
// only place that does.
//
// Two shell scripts read this file - the bootstrap beside a workspace and the
// filler before a job - and each had its own `read -r` list. When the three
// level fields merged into one pre-ordered directory, one list was updated and
// the other was not: its assignments shifted by two columns, the source path
// came out empty, and every file in an isolated workspace was reported missing.
// Deriving both lists from this one makes that drift impossible rather than
// merely unlikely.
var extractManifestFields = []string{"extract", "dataset", "dir", "path", "feuille"}

// extractManifestReadLine is the shell that unpacks one record.
func extractManifestReadLine() string {
	return "IFS='\t' read -r " + strings.Join(extractManifestFields, " ")
}

type extractMountEntry struct {
	ExtractName string
	DatasetDir  string
	// Dir is where this file sits under the extract, levels already in the
	// order the extract asked for.
	//
	// The three levels used to be separate fields and the shell joined them in
	// a fixed order, which is why the tree could only ever be subject-first.
	// Composing the directory here makes the layout a property of the extract
	// instead of a line of the bootstrap script - and leaves the shell with
	// one less thing to get right.
	Dir  string
	Path string
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
		colonnes := []string{entry.ExtractName, entry.DatasetDir, entry.Dir, entry.Path, entry.Leaf}
		fmt.Fprintln(writer, strings.Join(colonnes, "\t"))
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
	root := workspaceExtractsPath
	lines := legacyExtractTreeLines(projectMountPath)
	// Cree meme quand il n'y a rien a y mettre.
	//
	// L'espace de travail VS Code declare ce dossier, et un dossier declare
	// mais absent faisait echouer le scan de demarrage de l'assistant. Une
	// image d'environnement d'avant ce chemin ne le porte pas, donc le script
	// le cree - avec repli par sudo, qui echoue sans bruit quand il n'y est
	// pas, puisque l'entree de l'espace de travail se retire alors d'elle-meme.
	lines = append(lines, extractRootLines(root)...)
	lines = append(lines, extractTreeLines(root, hasManifest, refusedCount)...)
	return lines
}

// legacyExtractTreeLines gets the old tree out of the way, once.
//
// The tree moved from <project>/extracts to /extracts, and nothing moved what
// was already there. The project volume outlives the workspace, so every
// project that mounted an extract before the move still carries a full tree of
// working symlinks under /mnt/extracts - in the fixed subject/visit/modality
// order, because that is all the old code could build.
//
// That is worse than clutter. The links resolve, so the directory does not
// look stale: somebody opens /mnt/extracts, reads a differently shaped and
// frozen-in-time view of the same study, and has no way to know. A researcher
// found it before we did.
//
// Renamed, never deleted, and only when it holds nothing but directories and
// symlinks. Whoever put their own files in a directory of that name keeps
// them, and an operator who wants the old tree back has it.
func legacyExtractTreeLines(projectMountPath string) []string {
	mount := strings.TrimRight(strings.TrimSpace(projectMountPath), "/")
	if mount == "" || mount == "/" {
		return nil
	}
	legacy := mount + "/extracts"
	aside := mount + "/extracts.deplace-vers-slash-extracts"
	return []string{
		fmt.Sprintf("if [ -d %s ] && [ ! -e %s ]; then", shellQuote(legacy), shellQuote(aside)),
		// -not -type d -not -type l : anything that is neither a directory nor
		// a symlink is somebody's own file, and this stops there.
		fmt.Sprintf("  if [ -z \"$(find %s -not -type d -not -type l -print -quit 2>/dev/null)\" ]; then", shellQuote(legacy)),
		fmt.Sprintf("    if mv %s %s 2>/dev/null; then", shellQuote(legacy), shellQuote(aside)),
		fmt.Sprintf("      echo '[bootstrap] the old extract tree under %s was moved aside to %s; extracts now live at %s'", legacy, aside, workspaceExtractsPath),
		"    fi",
		"  else",
		fmt.Sprintf("    echo '[bootstrap] %s holds files of your own and was left alone; extracts now live at %s'", legacy, workspaceExtractsPath),
		"  fi",
		"fi",
	}
}

// extractRootLines makes sure the directory exists at all.
//
// It sits at the container root, which belongs to root, so the shipped images
// create it and sudo covers an image that has not been rebuilt yet.
//
// It used to run only when an extract was being mounted, which left the
// directory missing on every workspace without one - while the VS Code
// workspace file declared it regardless, so the assistant failed its startup
// scan with "ENOENT: no such file or directory, scandir '/extracts'" on every
// launch. An empty folder is the signal that was wanted; an absent one is a
// stack trace.
func extractRootLines(root string) []string {
	return []string{
		fmt.Sprintf("if [ ! -d %s ]; then (mkdir -p %s 2>/dev/null || sudo mkdir -p %s 2>/dev/null || true); fi", shellQuote(root), shellQuote(root), shellQuote(root)),
		fmt.Sprintf("if [ -d %s ] && [ ! -w %s ]; then (sudo chown noryx:noryx %s 2>/dev/null || true); fi", shellQuote(root), shellQuote(root), shellQuote(root)),
	}
}

// extractTreeLines rebuilds the tree at every start: the links are cheap, and a
// workspace whose extract changed must not keep yesterday's shape.
func extractTreeLines(root string, hasManifest bool, refusedCount int) []string {
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
		fmt.Sprintf("  if [ ! -w %s ]; then echo '[bootstrap] %s is not writable: the extract was not mounted'; else", shellQuote(root), root),
		fmt.Sprintf("  rm -rf %s && mkdir -p %s", shellQuote(root), shellQuote(root)),
		// Links, never copies: the data stays in the dataset mount, which is
		// mounted read-only, and the extract is a second way of looking at it.
		"  base64 -d /var/run/noryx/bootstrap/extracts.b64 2>/dev/null | gunzip 2>/dev/null | while " + extractManifestReadLine() + "; do",
		fmt.Sprintf("    target=/datasets/\"$dataset\"/\"$path\"; cible=%s/\"$extract\"/\"$dir\"", shellQuote(root)),
		"    mkdir -p \"$cible\"/\"$(dirname \"$feuille\")\" 2>/dev/null || continue",
		"    ln -sfn \"$target\" \"$cible\"/\"$feuille\" 2>/dev/null || true",
		"  done",
		fmt.Sprintf("  echo \"[bootstrap] extract links ready: $(find %s -type l 2>/dev/null | wc -l) file(s)\"", shellQuote(root)),
		"  fi",
		"fi",
	}
}

// withoutDirectoryKeys drops the keys that are directories, not files.
//
// S3 has no directories, but a bucket filled by a tool that thinks it does
// carries zero-byte keys for them: SELENA-01-001/20260218/ANTERION/DICOM sits
// in the listing beside the files under it. The scan recorded them as objects
// and an extract froze them as members.
//
// Linking one is not merely untidy, it hides the study. The link
// `.../anterion/DICOM` points straight into the read-only dataset, so the
// `mkdir -p` that every file beneath it needs then fails against a symlink to
// a read-only directory, and each of those files is skipped. On EMSE on
// 2026-10-01 a 4,019-file extract of SELENA-01 mounted 186 files: 111
// directory keys had shadowed everything under them, and the tree showed the
// raw bucket layout inside each modality instead of the selection.
//
// A key is a directory when another key starts with it plus a separator.
// Sorting makes that a single pass: in lexicographic order, a prefix is
// immediately followed by what it prefixes.
func withoutDirectoryKeys(members []extractdomain.Member) []extractdomain.Member {
	if len(members) < 2 {
		return members
	}
	ordered := make([]extractdomain.Member, len(members))
	copy(ordered, members)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].Path < ordered[j].Path })

	kept := make([]extractdomain.Member, 0, len(ordered))
	for index, member := range ordered {
		path := strings.TrimSuffix(member.Path, "/")
		if path == "" {
			continue
		}
		if index+1 < len(ordered) && strings.HasPrefix(ordered[index+1].Path, path+"/") {
			continue
		}
		kept = append(kept, member)
	}
	return kept
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
		members = withoutDirectoryKeys(members)
		name := sanitizeWorkspacePathName(item.Name)
		for _, member := range members {
			subject := sanitizeWorkspacePathName(member.SubjectID)
			visit := sanitizeWorkspacePathName(member.Visit)
			modality := sanitizeWorkspacePathName(member.Modality)
			entries = append(entries, extractMountEntry{
				ExtractName: name,
				DatasetDir:  directory,
				Dir:         strings.Join(extractdomain.DirectoryFor(item.Layout, subject, visit, modality), "/"),
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

// extractMountFor prepares an extract tree for any workload, not just a
// workspace.
//
// A workspace could mount an extract and a job could not, which emptied the
// object of its purpose: an extract exists so that a calculation can be
// reproduced on exactly the files it ran on, and a job is what reproduces a
// calculation. Somebody wanting the same n from a job had to mount the whole
// dataset and re-filter in their code - which is the thing the extract was
// built to remove.
//
// So the preparation lives here, in one place, and the three workloads call it.
// The alternative was three copies of a manifest encoder, which is three
// chances to ship one that silently mounts a partial study.
type extractMount struct {
	// Manifest is the gzipped, base64 file list, empty when there is nothing
	// to mount or when the selection is too large to ship.
	Manifest string
	// Refused counts the files left out because the manifest did not fit. The
	// workload says so rather than building a partial tree, because a tree
	// missing files is a different study and looks like a complete one.
	Refused int
	// Names is what was mounted, for the record a job keeps.
	Names []string
}

func (h Handlers) extractMountFor(projectID string, attachedDatasets []workspaceAttachedDataset) extractMount {
	entries := h.extractMountEntries(projectID, attachedDatasets)
	if len(entries) == 0 {
		return extractMount{}
	}
	manifest, fits := encodeExtractManifest(entries)
	if !fits {
		return extractMount{Refused: len(entries)}
	}
	seen := map[string]bool{}
	names := []string{}
	for _, entry := range entries {
		if !seen[entry.ExtractName] {
			seen[entry.ExtractName] = true
			names = append(names, entry.ExtractName)
		}
	}
	return extractMount{Manifest: manifest, Names: names}
}

// extractSecretData is what the bootstrap secret carries, or nothing.
func extractSecretData(mount extractMount) map[string]string {
	if mount.Manifest == "" {
		return nil
	}
	return map[string]string{"extracts.b64": mount.Manifest}
}

// extractIsolation is the boundary of ADR-038, prepared for any workload.
//
// It existed for workspaces only, which made the guarantee a property of one
// screen rather than of the platform: a job in the same project mounted the
// whole bucket, so "the team sees the selection and nothing else" held while
// somebody typed and stopped the moment a calculation ran. A boundary with an
// exception is not a boundary.
//
// What it returns is the three pieces a workload needs: the tree to mount, the
// container that fills it, and the dataset volumes it must *not* mount.
type extractIsolation struct {
	// Tree is the cache subtree, read-only, mounted where the extract goes.
	Tree noryxruntime.PersistentVolumeClaimMount
	// Filler reads the buckets and writes the tree. For a workspace it runs
	// beside the main container, so somebody starts working on the first file
	// rather than the last. For a job or an application it runs before,
	// because half a selection is a different study and a wrong answer served
	// quickly is still wrong.
	Filler *noryxruntime.SidecarSpec
}

// prepareExtractIsolation builds it, or explains why it cannot.
func (h Handlers) prepareExtractIsolation(
	name string,
	image string,
	datasetVolumes []noryxruntime.PersistentVolumeClaimMount,
) (extractIsolation, error) {
	if err := h.ensureExtractCache(); err != nil {
		return extractIsolation{}, err
	}
	const cacheRoot = "/cache"
	return extractIsolation{
		// Its own tree, not the cache. The objects underneath are shared by
		// the whole installation, which is what makes a second team on the
		// same modality free; what a workload browses is its own.
		Tree: noryxruntime.PersistentVolumeClaimMount{
			ClaimName: extractCacheClaim,
			MountPath: workspaceExtractsPath,
			SubPath:   "trees/" + name,
			ReadOnly:  true,
		},
		Filler: &noryxruntime.SidecarSpec{
			Name:    "extract-filler",
			Image:   image,
			Command: []string{"/bin/sh", "-c"},
			Args:    []string{extractFillerScript(cacheRoot, name)},
			// Read-only on the source, whatever the caller's role on the
			// dataset: this container exists to copy out of it, and nothing it
			// can do should be able to write back.
			Volumes: append(readOnlyMounts(datasetVolumes),
				noryxruntime.PersistentVolumeClaimMount{ClaimName: extractCacheClaim, MountPath: cacheRoot}),
			Secrets: []noryxruntime.SecretMount{{
				SecretName: name + "-bootstrap",
				MountPath:  "/var/run/noryx/bootstrap",
				ReadOnly:   true,
			}},
			// It waits on the network far more than on the processor, so it
			// reserves almost nothing and may burst. Reserving what it may
			// peak at - which is what Kubernetes does when only a limit is
			// given - had it asking for twenty times the workspace beside it,
			// and the node refused them both.
			CPURequest: "100m",
			MemRequest: "128Mi",
			CPULimit:   "1",
			MemLimit:   "1Gi",
		},
	}, nil
}

// refuseEmptyIsolation reports whether isolating would leave nothing at all.
//
// A workload isolated to extracts it does not have is a workload with no data
// and no explanation, and the person concludes the platform lost their study.
func refuseEmptyIsolation(isolated bool, mount extractMount) bool {
	return isolated && mount.Manifest == ""
}
