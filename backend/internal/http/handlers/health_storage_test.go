package handlers

import (
	"strings"
	"testing"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

const gib = int64(1) << 30

type storageRuntime struct {
	noryxruntime.Runner
	capacity noryxruntime.StorageCapacity
}

func (r storageRuntime) StorageCapacity() (noryxruntime.StorageCapacity, error) {
	return r.capacity, nil
}

func alertsFor(nodes ...noryxruntime.StorageNode) []healthAlert {
	return Handlers{runtime: storageRuntime{capacity: noryxruntime.StorageCapacity{
		Available: true, Source: "longhorn", Nodes: nodes,
	}}}.storageCapacityAlerts()
}

// The state EMSE was in the morning nobody could start a workspace: 130 GiB
// claimed of 134 allocatable, and 143 GiB of disk sitting unused.
func TestRunningOutOfRoomToAllocateIsReportedBeforeAnybodyWaits(t *testing.T) {
	alerts := alertsFor(noryxruntime.StorageNode{
		Name: "noryx-ee-worker-1", Claimed: 130 * gib, Schedulable: 4 * gib,
		Maximum: 134 * gib, Free: 143 * gib,
	})
	if len(alerts) != 1 {
		t.Fatalf("expected one alert, got %d", len(alerts))
	}
	if alerts[0].Severity != healthCritical {
		t.Errorf("3%% of allocatable left should be critical, got %v", alerts[0].Severity)
	}
	// An administrator reading "4 GiB left" on a machine with 143 GiB free
	// concludes the alert is wrong and stops reading the next one. Both
	// numbers, and the sentence that reconciles them.
	if !strings.Contains(alerts[0].Detail, "143 GiB") {
		t.Errorf("free disk was not shown: %s", alerts[0].Detail)
	}
	if !strings.Contains(alerts[0].Detail, "not against free disk") {
		t.Errorf("nothing reconciles the two numbers: %s", alerts[0].Detail)
	}
}

func TestTenPercentLeftWarnsWithoutCryingCritical(t *testing.T) {
	alerts := alertsFor(noryxruntime.StorageNode{
		Name: "n1", Claimed: 91 * gib, Schedulable: 9 * gib, Maximum: 100 * gib, Free: 90 * gib,
	})
	if len(alerts) != 1 || alerts[0].Severity != healthWarning {
		t.Fatalf("expected one warning at 9%%, got %v", alerts)
	}
}

func TestPlentyOfRoomSaysNothing(t *testing.T) {
	if alerts := alertsFor(noryxruntime.StorageNode{
		Name: "n1", Claimed: 20 * gib, Schedulable: 80 * gib, Maximum: 100 * gib, Free: 95 * gib,
	}); len(alerts) != 0 {
		t.Fatalf("a healthy node was reported: %v", alerts)
	}
}

func TestTheWorstNodeDecides(t *testing.T) {
	// Volumes are placed on one node. An average across a healthy node and a
	// full one describes a cluster that does not exist.
	alerts := alertsFor(
		noryxruntime.StorageNode{Name: "roomy", Claimed: 10 * gib, Schedulable: 90 * gib, Maximum: 100 * gib, Free: 95 * gib},
		noryxruntime.StorageNode{Name: "full", Claimed: 99 * gib, Schedulable: 1 * gib, Maximum: 100 * gib, Free: 80 * gib},
	)
	if len(alerts) != 1 {
		t.Fatalf("the full node was averaged away: %v", alerts)
	}
	if !strings.Contains(alerts[0].Summary, "full") {
		t.Errorf("the wrong node was named: %s", alerts[0].Summary)
	}
}

func TestAnUnmeasurableStorageLayerIsNotAnIncident(t *testing.T) {
	// An installation on a storage layer this cannot ask still runs. Saying
	// "critical" about a reading that does not exist teaches an operator to
	// ignore the alert that matters.
	h := Handlers{runtime: storageRuntime{capacity: noryxruntime.StorageCapacity{
		Available: false, Detail: "no supported storage layer answered",
	}}}
	if alerts := h.storageCapacityAlerts(); len(alerts) != 0 {
		t.Fatalf("an unmeasurable reading produced an alert: %v", alerts)
	}
	// And a runtime that does not implement the reader at all is silent.
	if alerts := (Handlers{runtime: storageRuntime{}.Runner}).storageCapacityAlerts(); len(alerts) != 0 {
		t.Fatalf("a runtime without the capability produced an alert: %v", alerts)
	}
}
