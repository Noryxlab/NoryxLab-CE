package handlers

import (
	"fmt"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/health"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

// Warning an administrator before somebody cannot work.
//
// A PersistentVolumeClaim is accepted the moment it is written and fails much
// later, at attach time, with a message on a pod nobody is reading. So the
// platform running out of room to place volumes has no symptom until a person
// launches a workspace that never starts - which is how two people spent a
// morning waiting while the disks were 78% empty.
//
// The threshold is on what can still be *claimed*, not on free disk. Those are
// different numbers and the difference is the whole trap: a provisioner
// schedules on what volumes reserve, so a cluster with most of its disk empty
// can be unable to place one more volume. Reporting free disk here would
// reassure an administrator on exactly the morning they need warning.
const (
	// Ten percent left is a few workspaces: time to decide, not yet an
	// incident.
	storageHeadroomWarning = 0.10
	// Below this, the next launch is a coin toss.
	storageHeadroomCritical = 0.03
)

func (h Handlers) storageCapacityAlerts() []healthAlert {
	if h.runtime == nil {
		return nil
	}
	reader, ok := h.runtime.(noryxruntime.StorageCapacityReader)
	if !ok {
		return nil
	}
	capacity, err := reader.StorageCapacity()
	if err != nil || !capacity.Available || len(capacity.Nodes) == 0 {
		// Unmeasurable is not unhealthy. An installation on a storage layer
		// this cannot ask still runs; it just cannot be warned in advance, and
		// saying "critical" about a reading that does not exist would train an
		// operator to ignore this alert.
		return nil
	}

	// The worst node decides. Volumes are placed on one node, so an average
	// across a healthy one and a full one describes a cluster that does not
	// exist.
	worst := capacity.Nodes[0]
	worstRatio := headroomRatio(worst)
	for _, node := range capacity.Nodes[1:] {
		if ratio := headroomRatio(node); ratio < worstRatio {
			worst, worstRatio = node, ratio
		}
	}
	if worstRatio > storageHeadroomWarning {
		return nil
	}

	severity := healthWarning
	if worstRatio <= storageHeadroomCritical {
		severity = healthCritical
	}
	return []healthAlert{{
		Scope:    health.ScopePlatform,
		Severity: severity,
		Source:   "storage",
		Summary: fmt.Sprintf("%s has %s left to allocate (%.0f%% of what it may hand out)",
			worst.Name, humanBytes(worst.Schedulable), worstRatio*100),
		// Both numbers, because an administrator reading "8 GiB left" on a
		// machine with 140 GiB free will otherwise conclude the alert is wrong
		// and stop reading the next one.
		Detail: fmt.Sprintf(
			"%s already claimed of %s allocatable; %s of disk is actually unused. "+
				"Volumes are placed against what they claim, not against free disk.",
			humanBytes(worst.Claimed), humanBytes(worst.Maximum), humanBytes(worst.Free)),
		Action: "storage",
	}}
}

func headroomRatio(node noryxruntime.StorageNode) float64 {
	allocatable := node.Claimed + node.Schedulable
	if allocatable <= 0 {
		return 1
	}
	return float64(node.Schedulable) / float64(allocatable)
}

// humanBytes writes a size the way an administrator says it out loud.
func humanBytes(value int64) string {
	const unit = 1024
	if value < unit {
		return fmt.Sprintf("%d B", value)
	}
	sizes := []string{"KiB", "MiB", "GiB", "TiB", "PiB"}
	amount, exponent := float64(value)/unit, 0
	for amount >= unit && exponent < len(sizes)-1 {
		amount /= unit
		exponent++
	}
	formatted := fmt.Sprintf("%.1f", amount)
	formatted = strings.TrimSuffix(formatted, ".0")
	return formatted + " " + sizes[exponent]
}
