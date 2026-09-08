package handlers

import (
	"context"
	"log"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/usage"
)

// The usage sampler.
//
// It writes down what each project holds, every few minutes, so the platform
// can answer what a project consumed last month - not only what it is running
// now. That is the question behind cost, capacity and who pays for what.
//
// Sampling rather than integrating over lifecycles: a workspace stopped by a
// node failure leaves no end timestamp, and integrating records with missing
// ends produces a number that is confidently wrong. A sample claims only what
// it saw, and a missed sweep costs one interval instead of a whole run.
const (
	usageSampleInterval = 5 * time.Minute
	// A gap longer than this means the platform was not running, and crediting
	// it would invent consumption nobody had.
	usageMaxGap = 30 * time.Minute
	// Raw samples are kept for a quarter: long enough for a monthly bill and
	// the month before it, short enough that nobody has to think about the
	// size of the table. Consumption history is not an audit trail.
	usageRetention = 100 * 24 * time.Hour
)

func (h Handlers) StartUsageSampler(ctx context.Context) {
	if h.usageStore == nil || h.projectStore == nil {
		return
	}
	log.Printf("usage sampler started (every %s, kept %d days)", usageSampleInterval, int(usageRetention.Hours()/24))

	go func() {
		ticker := time.NewTicker(usageSampleInterval)
		defer ticker.Stop()
		// One sweep immediately, so a platform that restarts often still has
		// points rather than a series of gaps.
		h.sampleUsage()
		lastPrune := time.Now()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.sampleUsage()
				if time.Since(lastPrune) > 24*time.Hour {
					lastPrune = time.Now()
					if removed, err := h.usageStore.DeleteBefore(time.Now().UTC().Add(-usageRetention)); err == nil && removed > 0 {
						log.Printf("usage sampler pruned %d sample(s) older than %d days", removed, int(usageRetention.Hours()/24))
					}
				}
			}
		}
	}()
}

// sampleUsage measures every project at one instant. All of them share the
// same timestamp on purpose: two projects measured seconds apart would not add
// up to what the platform held.
func (h Handlers) sampleUsage() {
	projects, err := h.projectStore.List()
	if err != nil {
		log.Printf("usage sampler: cannot list projects: %v", err)
		return
	}
	at := time.Now().UTC().Truncate(time.Second)
	samples := make([]usage.Sample, 0, len(projects))
	for _, project := range projects {
		current, err := h.projectUsage(project.ID)
		if err != nil {
			continue
		}
		// A project holding nothing is not recorded. Writing zeroes for every
		// idle project every five minutes fills a table with the absence of
		// information, and the integration treats a missing sample as zero
		// anyway.
		if current.VCPU == 0 && current.MemoryGiB == 0 && current.Workspaces == 0 && current.Jobs == 0 && current.Apps == 0 {
			continue
		}
		samples = append(samples, usage.Sample{
			ProjectID:  project.ID,
			At:         at,
			VCPU:       current.VCPU,
			MemoryGiB:  current.MemoryGiB,
			Workspaces: current.Workspaces,
			Jobs:       current.Jobs,
			Apps:       current.Apps,
		})
	}
	if len(samples) == 0 {
		return
	}
	if err := h.usageStore.Record(samples); err != nil {
		log.Printf("usage sampler: cannot record %d sample(s): %v", len(samples), err)
	}
}
