package handlers

import (
	"context"
	"log"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

// How much each dataset holds, measured once a night.
//
// The home page used to walk every bucket while somebody waited for it, which
// is why that figure carried a deadline, gave up halfway on large datasets and
// left regulated ones out entirely. The exclusion was explained as a matter of
// not reading health data; it was really a matter of not having ten seconds.
// Summing object sizes reads no content - it is the same metadata a bucket
// listing returns - and out of band there is no page waiting.
//
// So every dataset is measured, regulated included, and the total the platform
// reports is finally the total.
const (
	// Nightly. The number moves when somebody uploads, which is not often
	// enough to justify walking terabytes more often, and a figure carrying
	// "as of last night" is honest in a way a stale cache is not.
	datasetSizeInterval = 24 * time.Hour
	// A sweep is allowed to take its time - nobody is waiting - but not
	// forever: a bucket that stopped answering must not hold the next sweep.
	datasetSizeTimeout = 30 * time.Minute
)

func (h Handlers) StartDatasetSizeSampler(ctx context.Context) {
	if h.datasetSizeStore == nil || h.datasetStore == nil {
		return
	}
	log.Printf("dataset size sampler started (every %s)", datasetSizeInterval)

	go func() {
		ticker := time.NewTicker(datasetSizeInterval)
		defer ticker.Stop()
		// Once at startup only when the figures are stale. A platform that
		// restarts three times during a deployment must not walk every bucket
		// three times.
		if h.datasetSizesAreStale() {
			h.measureDatasets(ctx)
		}
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.measureDatasets(ctx)
			}
		}
	}()
}

func (h Handlers) datasetSizesAreStale() bool {
	sizes, err := h.datasetSizeStore.List()
	if err != nil || len(sizes) == 0 {
		return true
	}
	newest := sizes[0].MeasuredAt
	for _, entry := range sizes[1:] {
		if entry.MeasuredAt.After(newest) {
			newest = entry.MeasuredAt
		}
	}
	return time.Since(newest) > datasetSizeInterval
}

func (h Handlers) measureDatasets(parent context.Context) {
	datasets, err := h.datasetStore.ListAll()
	if err != nil {
		log.Printf("dataset size sampler: %v", err)
		return
	}
	ctx, cancel := context.WithTimeout(parent, datasetSizeTimeout)
	defer cancel()

	started := time.Now()
	var total int64
	measured, failed := 0, 0
	for _, item := range datasets {
		entry := h.recordDatasetSize(ctx, item)
		if err := h.datasetSizeStore.Upsert(entry); err != nil {
			log.Printf("dataset size sampler: storing %s: %v", item.ID, err)
			continue
		}
		if entry.Failure != "" {
			failed++
			continue
		}
		measured++
		total += entry.Bytes
	}
	log.Printf("dataset sizes: %d measured, %d unreadable, %.1f GiB, in %s",
		measured, failed, float64(total)/(1<<30), time.Since(started).Round(time.Second))
}

// recordDatasetSize reuses the on-demand measurement rather than walking the
// bucket a second way.
//
// Two implementations of the same listing drift, and the one that drifts is
// always the one nobody watches. This one adds only what persistence needs:
// a failure written down instead of returned, and a truncated walk refused as
// a total.
func (h Handlers) recordDatasetSize(ctx context.Context, item dataset.Dataset) store.DatasetSize {
	entry := store.DatasetSize{DatasetID: item.ID, MeasuredAt: time.Now().UTC()}
	usage, err := h.measureDataset(ctx, item)
	switch {
	case err != nil:
		entry.Failure = err.Error()
	case usage.Truncated:
		// A walk that stopped counting has measured a floor, not a size.
		// Publishing it as a size is how a platform under-reports with
		// confidence.
		entry.Failure = "the listing stopped before the end of the bucket"
	default:
		entry.Bytes = usage.TotalBytes
		entry.Objects = int64(usage.Objects)
		entry.MeasuredAt = usage.MeasuredAt
	}
	return entry
}
