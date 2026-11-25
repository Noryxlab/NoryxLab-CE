package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	snapshotdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/snapshot"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/notify"
)

// Le croisement d'empreintes de nuit : la donnee reglementee qui a quitte son
// perimetre.
//
// C'est la contrepartie de platformrole.WriteBesideRegulatedData. La regle de
// co-montage (ADR-055) empeche la copie en retirant le droit d'ecrire a cote ;
// la fonction rend ce droit a qui en a besoin, et ce scan est ce qui reste
// quand on l'a rendu. Stephane, 2026-11-25 : « la ligne dans la matrice et le
// croisement d'empreintes de nuit avec une alerte ».
//
// Ce qu'il voit : un objet d'un dataset reglemente qui se retrouve dans un
// dataset non classe du meme projet. Ce qu'il ne voit pas, et qu'il faut
// savoir avant de s'y fier : un resultat *derive* (une agregation, un modele
// entraine) n'a l'empreinte de rien, une donnee ressortie par le reseau ou
// par le poste de la personne ne passe par aucun bucket, et une copie posee
// hors du projet - vers un dataset non monte dans le meme projet - n'est pas
// comparee. Il constate une recopie telle quelle, qui est le geste observe en
// production : 17 Go de PREMYOM1000 deposes dans un dataset personnel.
//
// Le perimetre est deliberement celui de la regle : les paires (dataset
// reglemente, dataset non classe) qui partagent un projet, c'est-a-dire les
// seules situations ou un workload a pu monter les deux a la fois. Croiser
// tous les buckets contre tous les autres couterait un listage complet de
// toute la plateforme chaque nuit pour repondre a une question que la
// topologie des projets a deja tranchee.
const (
	// Ce qu'une alerte nomme avant de dire « et N autres ».
	regulatedCopyScanNamed = 10
	// Au-dela de quoi on arrete de lire un dataset. Une liste S3 ramene mille
	// objets par appel : un dataset d'un million d'objets, c'est mille appels
	// et autant d'entrees en memoire pour une seule des deux faces du
	// croisement. On prefere le dire dans le journal plutot que faire tomber
	// l'API la nuit.
	regulatedCopyScanMaxObjects = 400_000
)

// scanRegulatedCopies compares every co-mounted pair and announces what it
// recognises on both sides.
func (h Handlers) scanRegulatedCopies(ctx context.Context) {
	if h.datasetStore == nil || h.projectResourceStore == nil {
		return
	}
	items, err := h.datasetStore.ListAll()
	if err != nil {
		log.Printf("regulated copy scan: cannot list datasets: %v", err)
		return
	}
	regulated, open := splitByRegulation(items)
	if len(regulated) == 0 || len(open) == 0 {
		return
	}

	// Les projets d'abord, les octets ensuite : sans paire co-montee il n'y a
	// aucun listage a payer.
	pairs := h.coMountedPairs(regulated, open)
	if len(pairs) == 0 {
		return
	}

	entries := map[string][]snapshotdomain.Entry{}
	read := func(item dataset.Dataset) ([]snapshotdomain.Entry, bool) {
		if known, cached := entries[item.ID]; cached {
			return known, len(known) > 0
		}
		client, _, err := h.datasetS3Client(item)
		if err != nil {
			log.Printf("regulated copy scan: no storage client for %s: %v", item.Name, err)
			entries[item.ID] = []snapshotdomain.Entry{}
			return nil, false
		}
		listCtx, cancel := context.WithTimeout(ctx, snapshotListTimeout)
		found, err := listDatasetEntries(listCtx, client, item.Bucket, item.Prefix)
		cancel()
		if err != nil {
			log.Printf("regulated copy scan: cannot list %s: %v", item.Name, err)
			entries[item.ID] = []snapshotdomain.Entry{}
			return nil, false
		}
		if len(found) > regulatedCopyScanMaxObjects {
			log.Printf("regulated copy scan: %s holds %d objects, above the %d limit; not compared",
				item.Name, len(found), regulatedCopyScanMaxObjects)
			entries[item.ID] = []snapshotdomain.Entry{}
			return nil, false
		}
		entries[item.ID] = found
		return found, len(found) > 0
	}

	for _, pair := range pairs {
		destination, ok := read(pair.open)
		if !ok {
			continue
		}
		source, ok := read(pair.regulated)
		if !ok {
			continue
		}
		copies := snapshotdomain.CrossMatch(source, destination)
		if len(copies) == 0 {
			continue
		}
		h.announceRegulatedCopies(pair, copies)
	}
}

// coMountedPair is one regulated dataset and one unclassified dataset that a
// single workload could have mounted together.
type coMountedPair struct {
	regulated dataset.Dataset
	open      dataset.Dataset
	projectID string
}

func (h Handlers) coMountedPairs(regulated, open []dataset.Dataset) []coMountedPair {
	projectsOf := map[string]map[string]bool{}
	lookup := func(datasetID string) map[string]bool {
		if known, cached := projectsOf[datasetID]; cached {
			return known
		}
		ids, err := h.projectResourceStore.ListDatasetProjectIDs(datasetID)
		if err != nil {
			log.Printf("regulated copy scan: cannot list projects of dataset %s: %v", datasetID, err)
			ids = nil
		}
		set := make(map[string]bool, len(ids))
		for _, id := range ids {
			if id = strings.TrimSpace(id); id != "" {
				set[id] = true
			}
		}
		projectsOf[datasetID] = set
		return set
	}

	pairs := []coMountedPair{}
	for _, source := range regulated {
		sourceProjects := lookup(source.ID)
		if len(sourceProjects) == 0 {
			continue
		}
		for _, destination := range open {
			shared := ""
			for projectID := range lookup(destination.ID) {
				if sourceProjects[projectID] && (shared == "" || projectID < shared) {
					shared = projectID
				}
			}
			if shared == "" {
				continue
			}
			pairs = append(pairs, coMountedPair{regulated: source, open: destination, projectID: shared})
		}
	}
	return pairs
}

// splitByRegulation keeps only what can be read: a dataset with no bucket is
// a declaration, not storage.
func splitByRegulation(items []dataset.Dataset) (regulated, open []dataset.Dataset) {
	for _, item := range items {
		if strings.TrimSpace(item.Bucket) == "" {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(item.Classification), "hds") {
			regulated = append(regulated, item)
			continue
		}
		open = append(open, item)
	}
	return regulated, open
}

func (h Handlers) announceRegulatedCopies(pair coMountedPair, copies []snapshotdomain.Copy) {
	identical, probable := 0, 0
	examples := make([]string, 0, regulatedCopyScanNamed)
	var bytes int64
	for _, item := range copies {
		switch item.Confidence {
		case snapshotdomain.CopyIdentical:
			identical++
		default:
			probable++
		}
		bytes += item.Size
		if len(examples) < regulatedCopyScanNamed {
			examples = append(examples, fmt.Sprintf("%s -> %s (%s)", item.RegulatedPath, item.OpenPath, item.Confidence))
		}
	}
	// Critique sans nuance : une copie reconnue hors perimetre est le seul
	// evenement de cette plateforme qui soit deja une fuite au moment ou on
	// le lit. Une derive de dataset laisse un doute, celle-ci non.
	h.notifier.SendAsync(notify.Alert{
		Severity: notify.SeverityCritical,
		Event:    "regulated.copy.detected",
		Summary: fmt.Sprintf("%d objet(s) du dataset reglemente %q se retrouvent dans %q, non classe",
			len(copies), pair.regulated.Name, pair.open.Name),
		Details: map[string]any{
			"regulatedDatasetId":   pair.regulated.ID,
			"regulatedDatasetName": pair.regulated.Name,
			"openDatasetId":        pair.open.ID,
			"openDatasetName":      pair.open.Name,
			"openDatasetOwner":     pair.open.OwnerUserID,
			"projectId":            pair.projectID,
			"identical":            identical,
			"probable":             probable,
			"totalSize":            bytes,
			"examples":             examples,
			// Ce que l'alerte ne dit pas, dit explicitement.
			"meaning": "objects of a regulated dataset were recognised in an unclassified one mounted by the same project; this says what and where, not who wrote them",
		},
	})
	log.Printf("regulated copy scan: %s -> %s, %d object(s) (%d identical, %d probable, %d bytes)",
		pair.regulated.Name, pair.open.Name, len(copies), identical, probable, bytes)
}
