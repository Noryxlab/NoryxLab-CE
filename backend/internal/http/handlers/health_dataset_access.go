package handlers

import (
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
)

// A dataset the platform can no longer read.
//
// Credentials get rotated. When the ones stored for a dataset stop working,
// nothing fails loudly: the dataset still appears in the list, still opens,
// still shows its name and description - it just cannot be read, and every
// figure computed over it quietly excludes it.
//
// On 2026-09-17 that had been true of one dataset for an unknown length of
// time. The only trace anywhere was a caption on the home page reporting
// "measured across 3 of 8 datasets, health datasets are not counted", which
// attributed the whole gap to a deliberate exclusion. The platform knew the
// difference - it counts unreadable separately from regulated - and said
// nothing with it.
//
// Regulated datasets are probed too, and the way they are probed is the point.
//
// The first version of this check skipped them, reasoning that walking the
// keys of health data to earn a green tick was a poor trade. That reasoning
// was right about listing and wrong about the conclusion, and it left the
// blind spot that hid this for half a day: the protection that stops the
// platform looking inside a regulated bucket also stopped it noticing it had
// lost the key.
//
// BucketExists settles it. It proves the endpoint, the credentials and access
// to that bucket, and it enumerates nothing - the platform can show it still
// holds the key without walking through the door.

const (
	// Short, and per dataset. This runs whenever somebody opens the health
	// screen, and an unreachable endpoint usually fails by not answering - a
	// generous deadline here is a health page that hangs.
	datasetProbeTimeout = 4 * time.Second
	// Past this the list stops naming them one by one and says how many.
	datasetNamesShown = 3
)

func (h Handlers) datasetAccessAlerts() []healthAlert {
	if h.datasetStore == nil {
		return nil
	}
	datasets, err := h.datasetStore.ListAll()
	if err != nil {
		// Unmeasurable is not unhealthy, and an alert about a reading that
		// does not exist teaches an operator to ignore this one.
		return nil
	}

	unreachable := []string{}
	for _, item := range datasets {
		if !h.datasetIsReachable(item) {
			name := strings.TrimSpace(item.Name)
			if name == "" {
				name = item.Bucket
			}
			unreachable = append(unreachable, name)
		}
	}
	if len(unreachable) == 0 {
		return nil
	}
	sort.Strings(unreachable)
	return []healthAlert{datasetAccessAlert(unreachable)}
}

// datasetAccessAlert builds the message, separately from finding the problem,
// so the wording an operator reads can be tested without an object store.
func datasetAccessAlert(unreachable []string) healthAlert {
	// A warning rather than critical: nobody's work stops, and the data is
	// still where it was. What has stopped is the platform's ability to see
	// it, which misleads every total computed over it until it is fixed.
	summary := "1 dataset cannot be read"
	if len(unreachable) > 1 {
		summary = strconv.Itoa(len(unreachable)) + " datasets cannot be read"
	}
	shown := unreachable
	suffix := ""
	if len(shown) > datasetNamesShown {
		shown = shown[:datasetNamesShown]
		suffix = ", and " + strconv.Itoa(len(unreachable)-datasetNamesShown) + " more"
	}
	return healthAlert{
		Severity: healthWarning,
		Source:   "datasets",
		Summary:  summary,
		Detail: "The platform cannot list " + strings.Join(shown, ", ") + suffix +
			". The usual cause is a credential that was rotated without being " +
			"updated here. Storage totals silently exclude it until it is fixed.",
	}
}

// datasetIsReachable asks the object store one question, with a deadline, and
// reads nothing.
func (h Handlers) datasetIsReachable(item dataset.Dataset) bool {
	client, _, err := h.datasetS3Client(item)
	if err != nil || client == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), datasetProbeTimeout)
	defer cancel()
	exists, err := client.BucketExists(ctx, item.Bucket)
	return err == nil && exists
}
