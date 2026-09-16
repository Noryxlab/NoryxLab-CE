package handlers

import (
	"net/http"
	"sort"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

// What the cluster has left to hand out.
//
// Kubernetes will not tell you this. A claim is accepted on sight and fails
// much later, at attach time, on a pod nobody is reading - so an installation
// can be one workspace away from refusing everybody with nothing on any screen
// saying so. This is the screen that says so.
//
// Two numbers, side by side and never merged: what volumes have *claimed*, and
// what disk is actually unused. They diverge enormously - a cluster can be 78%
// empty and unable to place one more volume - and an administrator shown only
// the second is being reassured on exactly the morning they need warning.

type storageCapacityNode struct {
	Name        string `json:"name"`
	Claimed     int64  `json:"claimed"`
	Schedulable int64  `json:"schedulable"`
	Allocatable int64  `json:"allocatable"`
	// FreeDisk is what is genuinely unused on the disk, which is a different
	// question and usually a far larger number.
	FreeDisk int64 `json:"freeDisk"`
	// HeadroomRatio is the share of what may still be claimed here. It is the
	// figure the health warning fires on.
	HeadroomRatio float64 `json:"headroomRatio"`
}

type storageCapacityReport struct {
	// Available false means the storage layer could not be asked. Every figure
	// is then absent rather than zero: a gauge drawn from zeroes reads as an
	// empty cluster, which is the opposite of the truth being reported.
	Available bool   `json:"available"`
	Source    string `json:"source,omitempty"`
	Detail    string `json:"detail,omitempty"`

	Nodes []storageCapacityNode `json:"nodes,omitempty"`

	TotalClaimed     int64 `json:"totalClaimed"`
	TotalSchedulable int64 `json:"totalSchedulable"`
	TotalAllocatable int64 `json:"totalAllocatable"`
	TotalFreeDisk    int64 `json:"totalFreeDisk"`

	// WarnBelow is the threshold the platform acts on, so the screen and the
	// alert cannot disagree about when to worry.
	WarnBelow float64 `json:"warnBelow"`
}

func (h Handlers) GetAdminStorageCapacity(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "storage capacity"); !ok {
		return
	}
	report := storageCapacityReport{WarnBelow: storageHeadroomWarning}

	reader, ok := h.runtime.(noryxruntime.StorageCapacityReader)
	if !ok {
		report.Detail = "this platform runs on a storage layer it cannot ask about capacity"
		writeJSON(w, http.StatusOK, report)
		return
	}
	capacity, err := reader.StorageCapacity()
	if err != nil {
		report.Detail = "the storage layer could not be reached"
		writeJSON(w, http.StatusOK, report)
		return
	}
	report.Available = capacity.Available
	report.Source = capacity.Source
	report.Detail = capacity.Detail

	for _, node := range capacity.Nodes {
		report.Nodes = append(report.Nodes, storageCapacityNode{
			Name:          node.Name,
			Claimed:       node.Claimed,
			Schedulable:   node.Schedulable,
			Allocatable:   node.Claimed + node.Schedulable,
			FreeDisk:      node.Free,
			HeadroomRatio: headroomRatio(node),
		})
		report.TotalClaimed += node.Claimed
		report.TotalSchedulable += node.Schedulable
		report.TotalAllocatable += node.Claimed + node.Schedulable
		report.TotalFreeDisk += node.Free
	}
	// Tightest first: the node about to refuse a volume is the one worth
	// reading, and it is rarely the first one the storage layer lists.
	sort.Slice(report.Nodes, func(i, j int) bool {
		return report.Nodes[i].HeadroomRatio < report.Nodes[j].HeadroomRatio
	})
	writeJSON(w, http.StatusOK, report)
}
