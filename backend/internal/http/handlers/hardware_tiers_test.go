package handlers

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDefaultHardwareTierUsesLowHiddenRequests(t *testing.T) {
	h := Handlers{hardwareTiers: defaultHardwareTiers()}
	tier, ok := h.resolveHardwareTier("")
	if !ok || tier.ID != "standard" {
		t.Fatal("expected standard default hardware tier")
	}
	if tier.CPURequest != "10m" || tier.MemoryRequest != "64Mi" {
		t.Fatal("default tier must use very low internal requests")
	}

	payload, err := json.Marshal(tier)
	if err != nil {
		t.Fatal(err)
	}
	if string(payload) == "" || strings.Contains(string(payload), "cpuRequest") || strings.Contains(string(payload), "memoryRequest") || strings.Contains(string(payload), "ephemeralRequest") {
		t.Fatal("hardware tier API must not expose internal requests")
	}
}

func TestUnknownHardwareTierIsRejected(t *testing.T) {
	h := Handlers{hardwareTiers: defaultHardwareTiers()}
	if _, ok := h.resolveHardwareTier("unlimited"); ok {
		t.Fatal("unknown hardware tier must be rejected")
	}
}
