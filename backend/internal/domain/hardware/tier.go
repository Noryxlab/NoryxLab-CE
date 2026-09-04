// Package hardware holds the machine sizes a workspace, app or job can run on.
//
// These used to be four sizes compiled into the binary, with their display
// names hardcoded again in the interface's French and English catalogues. An
// installation that needed a fifth size, or names that matched how a customer
// talks about their machines, needed a release. They are now records an
// administrator edits, the way Domino presents hardware tiers.
package hardware

// Tier is one machine size.
//
// Requests and limits are kept apart because they answer different questions:
// the request is what the scheduler reserves, and the limit is what the pod may
// reach. Setting them equal wastes a cluster; setting the request at zero
// oversubscribes it. Both are the administrator's call, so both are stored.
type Tier struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`

	CPURequest    string `json:"cpuRequest"`
	CPULimit      string `json:"cpuLimit"`
	MemoryRequest string `json:"memoryRequest"`
	MemoryLimit   string `json:"memoryLimit"`

	EphemeralStorageRequest string `json:"ephemeralStorageRequest"`
	EphemeralStorageLimit   string `json:"ephemeralStorageLimit"`

	// Default marks the tier preselected when a caller names none. Exactly one
	// tier holds it; the store is what enforces that.
	Default bool `json:"default"`
	// Position orders the list an administrator sees and a user picks from.
	Position int `json:"position"`
}

// Defaults are the sizes an installation starts with. They seed an empty
// store, so a platform that upgrades into this feature keeps the four sizes it
// already had, under the same ids - workspaces already running refer to them.
func Defaults() []Tier {
	return []Tier{
		{ID: "0.5x2", Name: "Découverte", Description: "0.5 vCPU · 2 Gi RAM", CPURequest: "100m", CPULimit: "500m", MemoryRequest: "64Mi", MemoryLimit: "2Gi", EphemeralStorageRequest: "64Mi", EphemeralStorageLimit: "4Gi", Position: 1},
		{ID: "1x4", Name: "Standard", Description: "1 vCPU · 4 Gi RAM", CPURequest: "100m", CPULimit: "1", MemoryRequest: "64Mi", MemoryLimit: "4Gi", EphemeralStorageRequest: "64Mi", EphemeralStorageLimit: "8Gi", Default: true, Position: 2},
		{ID: "2x8", Name: "Confortable", Description: "2 vCPU · 8 Gi RAM", CPURequest: "100m", CPULimit: "2", MemoryRequest: "64Mi", MemoryLimit: "8Gi", EphemeralStorageRequest: "64Mi", EphemeralStorageLimit: "16Gi", Position: 3},
		{ID: "4x16", Name: "Intensif", Description: "4 vCPU · 16 Gi RAM", CPURequest: "100m", CPULimit: "4", MemoryRequest: "64Mi", MemoryLimit: "16Gi", EphemeralStorageRequest: "64Mi", EphemeralStorageLimit: "32Gi", Position: 4},
	}
}
