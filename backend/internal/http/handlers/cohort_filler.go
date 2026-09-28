package handlers

import (
	"fmt"
	"strings"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

// Filling a cohort into the cache, beside a workspace that cannot see the
// bucket.
//
// The mounted-links arrangement is a view: the dataset is in the same
// container, so a shell walks out of the selection into everything. Cellar
// offers neither scoped credentials nor wildcard policies, so the boundary
// cannot live in the storage - it moves here, into which container mounts
// what.
//
// Measured on EMSE on 2026-09-26 before any of this was written. An ANTERION
// cohort of five subjects, 2 940 objects and 5.97 GiB: first readable file at
// zero seconds, last at 130, and 2 ms per read afterwards against 44 MB/s and
// no caching at all through the mount. Re-reading a file used to cost exactly
// what reading it the first time cost; that is what this ends.
//
// Three properties the script below exists to hold:
//
//   - A file appears only once it has landed. A tree of paths that error on
//     open is worse than a tree that grows, and a workspace that waits for the
//     whole cohort is a workspace nobody uses.
//   - Parallel, because parallelism is the whole difference: 14 MB/s in one
//     stream against 81 with several, measured the same evening.
//   - It never writes to the dataset. The source is mounted read-only in this
//     container and nowhere else.

const cohortFillerLanes = 8

// cohortFillerScript copies a cohort's objects into the cache and builds the
// tree as they land.
func cohortFillerScript(cacheRoot, treeName string) string {
	lines := []string{
		"#!/bin/sh",
		"set -u",
		"manifest=/var/run/noryx/bootstrap/cohorts.b64",
		// Objects are shared and the tree is not. The cache holds one copy of
		// each file for the whole installation - that is what makes a second
		// team on the same modality free - and each workspace browses its own
		// tree of hard links into it. Mounting the cache whole showed a
		// workspace every cohort ever filled, from every project, and the
		// filesystem's lost+found beside them.
		fmt.Sprintf("objets=%s/objects", shellQuote(cacheRoot)),
		fmt.Sprintf("root=%s/trees/%s", shellQuote(cacheRoot), shellQuote(treeName)),
		"if [ ! -f \"$manifest\" ]; then",
		"  echo '[filler] no cohort manifest; nothing to do'",
		"  exit 0",
		"fi",
		"mkdir -p \"$root\" 2>/dev/null",
		"debut=$(date +%s)",
		fmt.Sprintf("echo \"[filler] filling with %d lanes\"", cohortFillerLanes),
		"base64 -d \"$manifest\" 2>/dev/null | gunzip 2>/dev/null > /tmp/cohorts.tsv",
		"total=$(wc -l < /tmp/cohorts.tsv 2>/dev/null || echo 0)",
		"echo \"[filler] $total file(s) to place\"",
		"voie=0",
		fmt.Sprintf("while [ \"$voie\" -lt %d ]; do", cohortFillerLanes),
		"  (",
		"    n=0",
		"    while IFS='\t' read -r cohort dataset subject visit modality path; do",
		"      n=$((n+1))",
		fmt.Sprintf("      [ $(( (n-1) %% %d )) -eq \"$voie\" ] || continue", cohortFillerLanes),
		"      source=/datasets/\"$dataset\"/\"$path\"",
		"      blob=\"$objets\"/\"$dataset\"/\"$path\"",
		"      dossier=\"$root\"/\"$cohort\"/\"$subject\"/\"$visit\"/\"$modality\"",
		"      cible=\"$dossier\"/$(basename \"$path\")",
		"      [ -f \"$cible\" ] && continue",
		"      mkdir -p \"$dossier\" \"$(dirname \"$blob\")\" 2>/dev/null || continue",
		// Already cached by another workspace, or another cohort: link and
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
		"    done < /tmp/cohorts.tsv",
		"  ) &",
		"  voie=$((voie+1))",
		"done",
		"wait",
		"fin=$(date +%s)",
		"echo \"[filler] done in $((fin-debut))s\"",
		// The container stays alive so the workspace's pod does not go
		// Succeeded under it, and so a later cohort change can be filled by
		// restarting this one rather than the workspace.
		"while true; do sleep 3600; done",
	}
	return strings.Join(lines, "\n") + "\n"
}

// cohortCacheClaim is the one volume an installation caches into.
//
// One for the installation rather than one per project: objects are cached by
// their path, so two teams working on the same modality fetch them once. A
// whole-study ANTERION cohort measured 33.6 GiB, a PLEXELITE one 267 - the
// cache holds a working set, never a bucket, and eviction is what keeps that
// true. Until eviction exists the size is the guard, which is why it is a
// setting and not a constant.
const cohortCacheClaim = "noryx-cohort-cache"

func (h Handlers) ensureCohortCache() error {
	if h.runtime == nil {
		return nil
	}
	return h.runtime.CreatePersistentVolumeClaim(noryxruntime.PersistentVolumeClaimSpec{
		Name:             cohortCacheClaim,
		StorageClassName: h.cohortCacheClass,
		Size:             firstNonEmpty(h.cohortCacheSize, "50Gi"),
		// Shared by every workspace that mounts a cohort, and by the fillers
		// beside them, so it has to be writable from several nodes at once.
		AccessModes: []string{"ReadWriteMany"},
		Labels: map[string]string{
			"app.kubernetes.io/name": "noryx-cohort-cache",
		},
	})
}
