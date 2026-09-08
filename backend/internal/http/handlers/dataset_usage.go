package handlers

import (
	"context"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/minio/minio-go/v7"
)

// How big a dataset is.
//
// The catalogue showed a bucket path under the heading "storage" and no size
// anywhere, so the one question a person asks about a dataset - how much is in
// there - had no answer on the platform at all. Nothing stores it: an object
// store knows its size only by being listed, and HDS-For is 24,179 objects and
// 400 GB of it.
//
// So it is measured on demand and remembered for a while. The measurement is a
// listing and nothing else: sizes come from the object metadata, contents are
// never read, and nothing is ever written back.

type datasetUsage struct {
	Objects    int       `json:"objects"`
	TotalBytes int64     `json:"totalBytes"`
	Truncated  bool      `json:"truncated"`
	MeasuredAt time.Time `json:"measuredAt"`
}

// Long enough that opening the catalogue twice costs one listing, short enough
// that a figure on screen is not from another working day.
const datasetUsageTTL = 15 * time.Minute

var datasetUsageCache = struct {
	sync.Mutex
	entries map[string]datasetUsage
}{entries: map[string]datasetUsage{}}

func (h Handlers) GetDatasetUsage(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasetStore.GetByID(strings.TrimSpace(r.PathValue("datasetID")))
	if err != nil || !found || !h.canReadDataset(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "dataset not found"})
		return
	}

	datasetUsageCache.Lock()
	cached, hit := datasetUsageCache.entries[item.ID]
	datasetUsageCache.Unlock()
	if hit && time.Since(cached.MeasuredAt) < datasetUsageTTL {
		writeJSON(w, http.StatusOK, cached)
		return
	}

	usage, err := h.measureDataset(r.Context(), item)
	if err != nil {
		// A store that cannot be listed leaves the size unknown rather than
		// zero: "0 B" on a 400 GB dataset is worse than no answer.
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "dataset measurement failed: " + err.Error()})
		return
	}
	datasetUsageCache.Lock()
	datasetUsageCache.entries[item.ID] = usage
	datasetUsageCache.Unlock()
	writeJSON(w, http.StatusOK, usage)
}

// measureDataset lists a dataset and adds up what it finds. It reads object
// metadata - key and size - and never object contents.
func (h Handlers) measureDataset(ctx context.Context, item dataset.Dataset) (datasetUsage, error) {
	client, _, err := h.datasetS3Client(item)
	if err != nil {
		return datasetUsage{}, err
	}
	prefix := strings.Trim(item.Prefix, "/")
	if prefix != "" {
		prefix += "/"
	}
	listCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	usage := datasetUsage{MeasuredAt: time.Now().UTC()}
	for object := range client.ListObjects(listCtx, item.Bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if object.Err != nil {
			return datasetUsage{}, object.Err
		}
		usage.Objects++
		usage.TotalBytes += object.Size
		if usage.Objects >= ontologyScanMaxObjects {
			// Said rather than hidden: a total that stopped counting must not
			// read as the size of the dataset.
			usage.Truncated = true
			break
		}
	}
	return usage, nil
}
