package handlers

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	snapshotdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/snapshot"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/notify"
)

// Le scan de nuit : ce qui a change dans un dataset sans passer par l'API.
//
// Decide le 2026-11-17, en meme temps que la regle de co-montage, et sans
// moyen de le construire jusqu'ici. Le raisonnement n'a pas bouge : une
// ecriture a travers un montage FUSE ne passe pas par l'API, donc le journal
// d'audit ne la voit pas. On ne peut pas l'empecher - un chercheur a le droit
// d'ecrire dans un dataset qu'il peut ecrire - mais on peut cesser de
// l'ignorer. « On ne previent pas, on constate. »
//
// Il ne compare que les datasets dont quelqu'un a pris un instantane. C'est
// deliberé : prendre une empreinte de tous les buckets chaque nuit coûterait
// un listage complet de toute la donnee de la plateforme pour repondre a une
// question que personne n'a posee sur la plupart d'entre eux. L'instantane
// est la facon de dire « celui-la, je veux le savoir ».
const (
	snapshotScanInterval = 24 * time.Hour
	// Ce qu'une alerte nomme avant de dire « et N autres ». Une alerte qui
	// deroule quatre mille chemins n'est pas lue.
	snapshotScanNamed = 10
)

func (h Handlers) StartSnapshotScan(ctx context.Context) {
	if h.snapshotStore == nil || h.datasetStore == nil || h.minioClient == nil {
		return
	}
	log.Printf("snapshot scan started (every %s, datasets with a snapshot only)", snapshotScanInterval)
	go func() {
		ticker := time.NewTicker(snapshotScanInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.scanSnapshots(ctx)
			}
		}
	}()
}

func (h Handlers) scanSnapshots(ctx context.Context) {
	datasets, err := h.datasetStore.ListAll()
	if err != nil {
		log.Printf("snapshot scan: cannot list datasets: %v", err)
		return
	}
	for _, item := range datasets {
		if strings.TrimSpace(item.Bucket) == "" {
			continue
		}
		latest, found, err := h.snapshotStore.LatestSnapshot(item.ID)
		if err != nil || !found {
			continue
		}
		before, found, err := h.snapshotStore.GetSnapshotEntries(latest.ID, item.ID)
		if err != nil || !found {
			continue
		}
		listCtx, cancel := context.WithTimeout(ctx, snapshotListTimeout)
		now, err := h.listDatasetEntries(listCtx, item.Bucket, item.Prefix)
		cancel()
		if err != nil {
			log.Printf("snapshot scan: cannot list %s: %v", item.Name, err)
			continue
		}
		changes := snapshotdomain.Compare(before, now)
		if len(changes) == 0 {
			continue
		}
		h.announceDrift(item.ID, item.Name, item.Classification, latest, changes)
	}
}

func (h Handlers) announceDrift(datasetID, datasetName, classification string, latest snapshotdomain.Snapshot, changes []snapshotdomain.Change) {
	added, removed, modified := 0, 0, 0
	names := make([]string, 0, snapshotScanNamed)
	for _, change := range changes {
		switch change.Kind {
		case snapshotdomain.Added:
			added++
		case snapshotdomain.Removed:
			removed++
		case snapshotdomain.Modified:
			modified++
		}
		if len(names) < snapshotScanNamed {
			names = append(names, change.Kind+" "+change.Path)
		}
	}
	// La donnee reglementee monte d'un cran, parce que la question qu'elle
	// pose n'est pas la meme : sur un dataset personnel une derive est une
	// journee de travail, sur une cohorte c'est une preuve qui a bouge.
	severity := notify.SeverityWarning
	if strings.EqualFold(strings.TrimSpace(classification), "hds") {
		severity = notify.SeverityCritical
	}
	h.notifier.SendAsync(notify.Alert{
		Severity: severity,
		Event:    "dataset.drift.detected",
		Summary: fmt.Sprintf("Dataset %q : %d objet(s) ont change depuis l'instantane du %s",
			datasetName, len(changes), latest.TakenAt.Format("2006-01-02")),
		Details: map[string]any{
			"datasetId":       datasetID,
			"datasetName":     datasetName,
			"classification":  classification,
			"snapshotId":      latest.ID,
			"snapshotTakenAt": latest.TakenAt,
			"added":           added,
			"removed":         removed,
			"modified":        modified,
			"examples":        names,
			// Ce que l'alerte ne dit pas, dit explicitement : une derive n'est
			// pas une intrusion. Quelqu'un qui a le droit d'ecrire a ecrit, et
			// c'est tout ce qu'on sait - le journal ne porte pas ces
			// ecritures, c'est precisement pourquoi ce scan existe.
			"meaning": "objects changed outside the API; this says what, not who",
		},
	})
	log.Printf("snapshot scan: %s drifted (+%d -%d ~%d) since %s",
		datasetName, added, removed, modified, latest.TakenAt.Format(time.RFC3339))
}
