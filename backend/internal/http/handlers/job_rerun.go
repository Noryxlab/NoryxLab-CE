//

package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	jobdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/job"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/platformrole"
)

// Rejouer un run, et dire en quoi le rejeu n'est pas identique.
//
// Le premier run enregistre desormais ce qu'il a lu et quel code il a execute
// (voir job_provenance.go). C'est ce qui rend ce bouton honnete : sans cet
// enregistrement, « rejouer » relancerait la definition d'aujourd'hui sous le
// nom d'hier - l'image que le tag designe maintenant, les datasets que le
// projet attache maintenant - et appellerait ca une reproduction.
//
// Deux choses ne sont volontairement PAS reprises du run d'origine.
//
// Les droits sont ceux de la personne qui appuie, jamais ceux du lanceur
// initial. Un rejeu qui rendrait la donnee du premier lanceur serait une
// elevation de privilege deguisee en commodite.
//
// Et le resultat est un nouveau run, pas une mise a jour de l'ancien. Un
// enregistrement de provenance qu'un rejeu ecraserait ne serait plus une
// trace.

// RerunJob relaunches a finished job from what that run recorded.
func (h Handlers) RerunJob(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	if !h.requireCapability(w, identity, platformrole.LaunchJob) {
		return
	}
	userID := identity.UserID()

	original, found, err := h.jobStore.GetByID(strings.TrimSpace(r.PathValue("jobID")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read job"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "job not found"})
		return
	}
	if !h.requireProjectRole(w, original.ProjectID, userID, actionLaunch, "job rerun") {
		return
	}

	// L'image par son empreinte, pas par son etiquette.
	//
	// C'est tout l'interet : une etiquette designe aujourd'hui ce qu'elle
	// designe aujourd'hui, et la reconstruction nocturne la fait bouger. Si
	// l'empreinte n'a pas ete enregistree - un run d'avant cette trace -, on
	// retombe sur l'etiquette et on le dit.
	image := strings.TrimSpace(original.Image)
	divergences := []string{}
	if digest := strings.TrimSpace(original.ImageDigest); digest != "" {
		image = imageAtDigest(original.Image, digest)
	} else {
		divergences = append(divergences, "image: no digest was recorded for the original run, so its tag is used and may have moved")
	}

	req := createJobRequest{
		ProjectID:    original.ProjectID,
		Name:         rerunName(original.Name),
		Image:        image,
		Command:      original.Command,
		Args:         original.Args,
		HardwareTier: original.HardwareTier,
	}
	_, _ = h.launchJob(w, r, identity, userID, req, original.ID)
}

// imageAtDigest replaces a tag by the digest that actually ran.
func imageAtDigest(image, digest string) string {
	image = strings.TrimSpace(image)
	digest = strings.TrimSpace(digest)
	if image == "" || digest == "" {
		return image
	}
	if strings.Contains(image, "@") {
		return image
	}
	// Le dernier ':' n'est une etiquette que s'il suit le dernier '/' : un
	// registre peut porter un port, et `harbor.lan:5000/x` n'a pas d'etiquette.
	if colon := strings.LastIndex(image, ":"); colon > strings.LastIndex(image, "/") {
		image = image[:colon]
	}
	return image + "@" + digest
}

// rerunName keeps the original name recognisable without growing a suffix at
// every replay - "analyse (rejeu)" replayed twice is still "analyse (rejeu)".
func rerunName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if strings.HasSuffix(name, " (rejeu)") {
		return name
	}
	return name + " (rejeu)"
}

// compareProvenance says how a replay differs from what it replays.
//
// Named rather than counted, and sorted so two readings of the same pair of
// runs produce the same list.
func compareProvenance(original jobdomain.Job, datasets []jobdomain.Dataset, repositories []jobdomain.Repository) []string {
	out := []string{}

	avant := map[string]string{}
	for _, item := range original.Datasets {
		avant[item.Name] = item.ID
	}
	maintenant := map[string]string{}
	for _, item := range datasets {
		maintenant[item.Name] = item.ID
	}
	for name := range avant {
		if _, present := maintenant[name]; !present {
			out = append(out, fmt.Sprintf("dataset %q was mounted by the original run and is not mounted now", name))
		}
	}
	for name := range maintenant {
		if _, present := avant[name]; !present {
			out = append(out, fmt.Sprintf("dataset %q is mounted now and was not mounted by the original run", name))
		}
	}

	commits := map[string]string{}
	for _, item := range original.Repositories {
		commits[item.Name] = item.Commit
	}
	for _, item := range repositories {
		before, known := commits[item.Name]
		switch {
		case !known:
			out = append(out, fmt.Sprintf("repository %q is attached now and was not recorded on the original run", item.Name))
		case before == "" || item.Commit == "":
			out = append(out, fmt.Sprintf("repository %q: one of the two commits is unknown, so the code cannot be compared", item.Name))
		case before != item.Commit:
			out = append(out, fmt.Sprintf("repository %q moved from %s to %s", item.Name, short(before), short(item.Commit)))
		}
	}
	sort.Strings(out)
	return out
}

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}
