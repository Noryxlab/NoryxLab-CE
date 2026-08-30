package handlers

import (
	"context"
	"log"
	"time"
)

// Job watcher.
//
// Job status was refreshed from the runtime only while serving a request, so a
// scheduled job failing at 03:00 stayed unnoticed until somebody opened the
// page the next morning. That is the gap this closes: the sweep runs on its
// own, and the transition into failure is what raises an alert (ADR-034).
const jobWatcherInterval = 2 * time.Minute

// StartJobWatcher refreshes job status in the background until ctx is
// cancelled, so a failure is observed when it happens rather than when
// somebody looks.
func (h Handlers) StartJobWatcher(ctx context.Context) {
	if h.jobStore == nil {
		return
	}
	log.Printf("job watcher started (every %s)", jobWatcherInterval)

	go func() {
		ticker := time.NewTicker(jobWatcherInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// syncJobsFromRuntime raises the alerts as it observes
				// transitions, so nothing else is needed here.
				h.syncJobsFromRuntime()
			}
		}
	}()
}
