package k8s

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

// Asking the storage layer how much room is left.
//
// Longhorn, because that is what both installations run and because it is the
// only party that knows: a PersistentVolumeClaim is accepted the moment it is
// written and fails at attach time, so nothing in the core API can answer this
// before somebody is already waiting.
//
// Every failure is an unavailable reading rather than an error. An
// installation on another storage class, or one that did not grant the read,
// is not broken - it simply cannot be warned in advance, and the interface has
// to say that instead of drawing an empty gauge.
const longhornNamespace = "longhorn-system"

func (r *Runtime) StorageCapacity() (noryxruntime.StorageCapacity, error) {
	body, err := r.get(fmt.Sprintf(
		"/apis/longhorn.io/v1beta2/namespaces/%s/nodes", longhornNamespace))
	if err != nil {
		return noryxruntime.StorageCapacity{
			Available: false,
			Detail:    "no supported storage layer answered: " + shortReason(err),
		}, nil
	}

	var response struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Spec struct {
				Disks map[string]struct {
					StorageReserved int64 `json:"storageReserved"`
				} `json:"disks"`
			} `json:"spec"`
			Status struct {
				DiskStatus map[string]struct {
					StorageAvailable int64 `json:"storageAvailable"`
					StorageMaximum   int64 `json:"storageMaximum"`
					StorageScheduled int64 `json:"storageScheduled"`
				} `json:"diskStatus"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return noryxruntime.StorageCapacity{
			Available: false, Detail: "the storage layer answered something unreadable",
		}, nil
	}

	// How much a new volume may still claim, which is not how much disk is
	// free: the provisioner allows claims up to a multiple of the disk it
	// manages, minus what it keeps in reserve, minus what is already claimed.
	overProvisioning := r.longhornOverProvisioning()

	out := noryxruntime.StorageCapacity{Available: false, Source: "longhorn"}
	for _, item := range response.Items {
		for name, disk := range item.Status.DiskStatus {
			reserved := item.Spec.Disks[name].StorageReserved
			allowed := int64(float64(disk.StorageMaximum-reserved) * overProvisioning / 100)
			schedulable := allowed - disk.StorageScheduled
			if schedulable < 0 {
				schedulable = 0
			}
			out.Available = true
			out.Nodes = append(out.Nodes, noryxruntime.StorageNode{
				Name:        item.Metadata.Name,
				Schedulable: schedulable,
				Claimed:     disk.StorageScheduled,
				Maximum:     disk.StorageMaximum - reserved,
				Free:        disk.StorageAvailable,
			})
		}
	}
	if !out.Available {
		out.Detail = "the storage layer reported no disk"
	}
	return out, nil
}

// longhornOverProvisioning reads the setting that decides how far claims may
// exceed the disk. Defaults to 100 - no over-provisioning - because assuming
// more would make the platform report room it has not been granted.
func (r *Runtime) longhornOverProvisioning() float64 {
	body, err := r.get(fmt.Sprintf(
		"/apis/longhorn.io/v1beta2/namespaces/%s/settings/storage-over-provisioning-percentage",
		longhornNamespace))
	if err != nil {
		return 100
	}
	var setting struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(body, &setting); err != nil {
		return 100
	}
	parsed, err := strconv.ParseFloat(strings.TrimSpace(setting.Value), 64)
	if err != nil || parsed <= 0 {
		return 100
	}
	return parsed
}

// shortReason keeps an API error readable in a sentence an administrator sees.
func shortReason(err error) string {
	message := strings.Join(strings.Fields(err.Error()), " ")
	if len(message) > 120 {
		return message[:120] + "…"
	}
	return message
}
