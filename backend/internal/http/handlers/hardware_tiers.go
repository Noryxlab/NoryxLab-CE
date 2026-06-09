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
		{ID: "0.5x2", Name: "0.5x2", Description: "0.5 vCPU · 2 Gi RAM", CPULimit: "500m", MemoryLimit: "2Gi", EphemeralStorageLimit: "4Gi", CPURequest: "100m", MemoryRequest: "64Mi", EphemeralRequest: "64Mi"},
		{ID: "1x4", Name: "1x4", Description: "1 vCPU · 4 Gi RAM", CPULimit: "1", MemoryLimit: "4Gi", EphemeralStorageLimit: "8Gi", CPURequest: "100m", MemoryRequest: "64Mi", EphemeralRequest: "64Mi", Default: true},
		{ID: "2x8", Name: "2x8", Description: "2 vCPU · 8 Gi RAM", CPULimit: "2", MemoryLimit: "8Gi", EphemeralStorageLimit: "16Gi", CPURequest: "100m", MemoryRequest: "64Mi", EphemeralRequest: "64Mi"},
		{ID: "4x16", Name: "4x16", Description: "4 vCPU · 16 Gi RAM", CPULimit: "4", MemoryLimit: "16Gi", EphemeralStorageLimit: "32Gi", CPURequest: "100m", MemoryRequest: "64Mi", EphemeralRequest: "64Mi"},
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
