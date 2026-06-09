package handlers

import (
	"net/http"
	"strings"
)

type hardwareTier struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Description           string `json:"description,omitempty"`
	CPULimit              string `json:"cpuLimit"`
	MemoryLimit           string `json:"memoryLimit"`
	EphemeralStorageLimit string `json:"ephemeralStorageLimit"`
	Default               bool   `json:"default"`
	CPURequest            string `json:"-"`
	MemoryRequest         string `json:"-"`
	EphemeralRequest      string `json:"-"`
}

func defaultHardwareTiers() []hardwareTier {
	return []hardwareTier{
		{ID: "small", Name: "Small", Description: "Exploration légère", CPULimit: "500m", MemoryLimit: "1Gi", EphemeralStorageLimit: "4Gi", CPURequest: "10m", MemoryRequest: "64Mi", EphemeralRequest: "64Mi"},
		{ID: "standard", Name: "Standard", Description: "Usage data science courant", CPULimit: "1", MemoryLimit: "2Gi", EphemeralStorageLimit: "8Gi", CPURequest: "10m", MemoryRequest: "64Mi", EphemeralRequest: "64Mi", Default: true},
		{ID: "medium", Name: "Medium", Description: "Calcul et mémoire intermédiaires", CPULimit: "2", MemoryLimit: "4Gi", EphemeralStorageLimit: "16Gi", CPURequest: "10m", MemoryRequest: "64Mi", EphemeralRequest: "64Mi"},
		{ID: "large", Name: "Large", Description: "Charges intensives", CPULimit: "4", MemoryLimit: "8Gi", EphemeralStorageLimit: "32Gi", CPURequest: "10m", MemoryRequest: "64Mi", EphemeralRequest: "64Mi"},
	}
}

func (h Handlers) GetHardwareTiers(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireIdentity(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": h.hardwareTiers})
}

func (h Handlers) resolveHardwareTier(raw string) (hardwareTier, bool) {
	requested := strings.TrimSpace(raw)
	for _, tier := range h.hardwareTiers {
		if requested != "" && strings.EqualFold(tier.ID, requested) {
			return tier, true
		}
	}
	if requested != "" {
		return hardwareTier{}, false
	}
	for _, tier := range h.hardwareTiers {
		if tier.Default {
			return tier, true
		}
	}
	if len(h.hardwareTiers) > 0 {
		return h.hardwareTiers[0], true
	}
	return hardwareTier{}, false
}
