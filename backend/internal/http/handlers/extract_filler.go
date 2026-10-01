package handlers

import (
	"fmt"
	"strings"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

// Filling an extract into the cache, beside a workspace that cannot see the
// bucket.
//
// The mounted-links arrangement is a view: the dataset is in the same
// container, so a shell walks out of the selection into everything. Cellar
// offers neither scoped credentials nor wildcard policies, so the boundary
// cannot live in the storage - it moves here, into which container mounts
// what.
//
// Measured on EMSE on 2026-09-26 before any of this was written. An ANTERION
// extract of five subjects, 2 940 objects and 5.97 GiB: first readable file at
// zero seconds, last at 130, and 2 ms per read afterwards against 44 MB/s and
// no caching at all through the mount. Re-reading a file used to cost exactly
// what reading it the first time cost; that is what this ends.
//
// Three properties the script below exists to hold:
//
//   - A file appears only once it has landed. A tree of paths that error on
//     open is worse than a tree that grows, and a workspace that waits for the
//     whole extract is a workspace nobody uses.
//   - Parallel, because parallelism is the whole difference: 14 MB/s in one
//     stream against 81 with several, measured the same evening.
//   - It never writes to the dataset. The source is mounted read-only in this
//     container and nowhere else.

const (
	extractFillerLanes = 8
	// The marks that bound the cache. Eviction starts at the high one and
	// stops at the low one rather than at the high one, so a launch frees a
	// useful amount instead of one file, and the next launch does not start
	// by evicting again.
	extractCacheHighMark = 85
	extractCacheLowMark  = 70
)

// extractFillerScript copies an extract's objects into the cache and builds the
// tree as they land.
func extractFillerScript(cacheRoot, treeName string) string {
	lines := []string{
		"#!/bin/sh",
		"set -u",
		"manifest=/var/run/noryx/bootstrap/extracts.b64",
		// Objects are shared and the tree is not. The cache holds one copy of
		// each file for the whole installation - that is what makes a second
		// team on the same modality free - and each workspace browses its own
		// tree of hard links into it. Mounting the cache whole showed a
		// workspace every extract ever filled, from every project, and the
		// filesystem's lost+found beside them.
		fmt.Sprintf("objets=%s/objects", shellQuote(cacheRoot)),
		fmt.Sprintf("root=%s/trees/%s", shellQuote(cacheRoot), shellQuote(treeName)),
		// The lease lives beside the tree, never inside it. Put in the tree it
		// became a file in somebody's extract directory - 64 files where the
		// extract holds 63, which is exactly the kind of detail that makes a
		// person doubt the rest.
		fmt.Sprintf("baux=%s/leases", shellQuote(cacheRoot)),
		fmt.Sprintf("bail=\"$baux\"/%s", shellQuote(treeName)),
		"mkdir -p \"$baux\" 2>/dev/null",
		"if [ ! -f \"$manifest\" ]; then",
		"  echo '[filler] no extract manifest; nothing to do'",
		"  exit 0",
		"fi",
		"mkdir -p \"$root\" 2>/dev/null",
		"touch \"$bail\" 2>/dev/null",
		// Whatever the last filler left behind, reaped by the next one to
		// start. Two hours of grace against a ten-minute lease: a tree is only
		// removed when nothing has renewed it for twelve intervals.
		fmt.Sprintf("for arbre in %s/trees/*; do", shellQuote(cacheRoot)),
		"  [ -d \"$arbre\" ] || continue",
		"  [ \"$arbre\" = \"$root\" ] && continue",
		"  if [ -z \"$(find \"$baux\"/\"$(basename \"$arbre\")\" -mmin -120 2>/dev/null)\" ]; then",
		"    echo \"[filler] arbre abandonne retire : $(basename \"$arbre\")\"",
		"    rm -rf \"$arbre\" \"$baux\"/\"$(basename \"$arbre\")\" 2>/dev/null",
		"  fi",
		"done",
		"debut=$(date +%s)",
		// Eviction, here, because the cache only grows when a workspace
		// launches - so this is when it is worth pruning, and it needs no
		// second component, no image to keep and no schedule to forget. A
		// cache nobody launches into stays full, which costs nothing: nobody
		// wants the space either.
		//
		// Only objects no tree references: a link count of one means the copy
		// in objects/ and nothing else. Oldest first, down to the low mark, so
		// a sweep frees a useful amount instead of one file per launch.
		"utilise=$(df -P \"$objets\" 2>/dev/null | awk 'NR==2 {print $5+0}')",
		fmt.Sprintf("if [ \"${utilise:-0}\" -gt %d ]; then", extractCacheHighMark),
		fmt.Sprintf("  echo \"[filler] cache a ${utilise}%%, elagage vers %d%%\"", extractCacheLowMark),
		"  find \"$objets\" -type f -links 1 -printf '%T@ %s %p\\n' 2>/dev/null | sort -n | while read -r quand taille chemin; do",
		"    reste=$(df -P \"$objets\" 2>/dev/null | awk 'NR==2 {print $5+0}')",
		fmt.Sprintf("    [ \"${reste:-0}\" -le %d ] && break", extractCacheLowMark),
		"    rm -f \"$chemin\" 2>/dev/null",
		"  done",
		"  echo \"[filler] cache a $(df -P \"$objets\" 2>/dev/null | awk 'NR==2 {print $5+0}')%% apres elagage\"",
		"fi",
		fmt.Sprintf("echo \"[filler] filling with %d lanes\"", extractFillerLanes),
		"base64 -d \"$manifest\" 2>/dev/null | gunzip 2>/dev/null > /tmp/extracts.tsv",
		"total=$(wc -l < /tmp/extracts.tsv 2>/dev/null || echo 0)",
		"echo \"[filler] $total file(s) to place\"",
		"voie=0",
		fmt.Sprintf("while [ \"$voie\" -lt %d ]; do", extractFillerLanes),
		"  (",
		"    n=0",
		"    while IFS='\t' read -r extract dataset subject visit modality path feuille; do",
		"      n=$((n+1))",
		fmt.Sprintf("      [ $(( (n-1) %% %d )) -eq \"$voie\" ] || continue", extractFillerLanes),
		"      source=/datasets/\"$dataset\"/\"$path\"",
		"      blob=\"$objets\"/\"$dataset\"/\"$path\"",
		"      dossier=\"$root\"/\"$extract\"/\"$subject\"/\"$visit\"/\"$modality\"",
		"      cible=\"$dossier\"/\"$feuille\"",
		"      [ -f \"$cible\" ] && continue",
		"      mkdir -p \"$(dirname \"$cible\")\" \"$(dirname \"$blob\")\" 2>/dev/null || continue",
		// Already cached by another workspace, or another extract: link and
		// move on. This is the line that makes the second team free.
		"      if [ -f \"$blob\" ]; then ln -f \"$blob\" \"$cible\" 2>/dev/null && continue; fi",
		// Written aside and moved into place, so a reader never opens a file
		// that is still arriving. The rename is what makes "appears once it
		// has landed" true rather than nearly true.
		"      if cp \"$source\" \"$blob\".partiel 2>/dev/null; then",
		"        mv \"$blob\".partiel \"$blob\" 2>/dev/null",
		"        ln -f \"$blob\" \"$cible\" 2>/dev/null",
		"      else",
		"        rm -f \"$blob\".partiel 2>/dev/null",
		"        echo \"[filler] MANQUE $dataset/$path\"",
		"      fi",
		"    done < /tmp/extracts.tsv",
		"  ) &",
		"  voie=$((voie+1))",
		"done",
		"wait",
		"fin=$(date +%s)",
		"echo \"[filler] done in $((fin-debut))s\"",
		// The container stays alive so the workspace's pod does not go
		// Succeeded under it, and so a later extract change can be filled by
		// restarting this one rather than the workspace.
		//
		// It also holds the lease on its own tree while it sleeps. A deleted
		// workspace leaves its tree of links behind - nothing here can ask
		// Kubernetes what still exists - and those links would keep objects
		// alive that nobody wants, so eviction would free nothing. Age alone
		// cannot say which tree is dead: a workspace open all day never
		// touches its own. A lease can.
		"while true; do touch \"$bail\" 2>/dev/null; sleep 600; done",
	}
	return strings.Join(lines, "\n") + "\n"
}

// extractCacheClaim is the one volume an installation caches into.
//
// One for the installation rather than one per project: objects are cached by
// their path, so two teams working on the same modality fetch them once. A
// whole-study ANTERION extract measured 33.6 GiB, a PLEXELITE one 267 - the
// cache holds a working set, never a bucket, and eviction is what keeps that
// true. Until eviction exists the size is the guard, which is why it is a
// setting and not a constant.
const extractCacheClaim = "noryx-extract-cache"

func (h Handlers) ensureExtractCache() error {
	if h.runtime == nil {
		return nil
	}
	return h.runtime.CreatePersistentVolumeClaim(noryxruntime.PersistentVolumeClaimSpec{
		Name:             extractCacheClaim,
		StorageClassName: h.extractCacheClass,
		Size:             firstNonEmpty(h.extractCacheSize, "50Gi"),
		// Shared by every workspace that mounts an extract, and by the fillers
		// beside them, so it has to be writable from several nodes at once.
		AccessModes: []string{"ReadWriteMany"},
		Labels: map[string]string{
			"app.kubernetes.io/name": "noryx-extract-cache",
		},
	})
}
