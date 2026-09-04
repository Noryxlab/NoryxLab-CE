package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

func TestDefaultHardwareTierUsesLowHiddenRequests(t *testing.T) {
	h := Handlers{hardwareTierStore: memory.NewHardwareTierStore()}
	tier, ok := h.resolveHardwareTier("")
	if !ok || tier.ID != "1x4" {
		t.Fatal("expected 1x4 default hardware tier")
	}
	if tier.CPURequest != "100m" || tier.MemoryRequest != "64Mi" {
		t.Fatal("default tier must use very low internal requests")
	}

	// The tier a user picks from must not carry the requests: they are how the
	// cluster is packed, not what the machine can do.
	recorder := httptest.NewRecorder()
	Handlers{hardwareTierStore: memory.NewHardwareTierStore(), authMode: "none"}.GetHardwareTiers(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/hardware-tiers", nil))
	body := recorder.Body.String()
	if strings.Contains(body, "cpuRequest") || strings.Contains(body, "memoryRequest") || strings.Contains(body, "ephemeralStorageRequest") {
		t.Fatalf("the public hardware tier list must not expose internal requests: %s", body)
	}
	payload, err := json.Marshal(tier)
	if err != nil || string(payload) == "" {
		t.Fatal("a tier must marshal for the administration screen")
	}
}

func TestUnknownHardwareTierIsRejected(t *testing.T) {
	h := Handlers{hardwareTierStore: memory.NewHardwareTierStore()}
	if _, ok := h.resolveHardwareTier("unlimited"); ok {
		t.Fatal("unknown hardware tier must be rejected")
	}
}

func TestHardwareTierValidationRefusesWhatKubernetesWouldRefuseLater(t *testing.T) {
	cases := []struct {
		name    string
		request hardwareTierRequest
		wants   string
	}{
		{
			name:    "a request above its limit",
			request: hardwareTierRequest{ID: "big", Name: "Big", CPURequest: "4", CPULimit: "2", MemoryRequest: "2Gi", MemoryLimit: "4Gi"},
			wants:   "cpuRequest cannot exceed cpuLimit",
		},
		{
			name:    "memory above its limit",
			request: hardwareTierRequest{ID: "big", Name: "Big", CPURequest: "1", CPULimit: "2", MemoryRequest: "8Gi", MemoryLimit: "4Gi"},
			wants:   "memoryRequest cannot exceed memoryLimit",
		},
		{
			name:    "a quantity that is not one",
			request: hardwareTierRequest{ID: "big", Name: "Big", CPURequest: "1", CPULimit: "2", MemoryRequest: "64", MemoryLimit: "4Gi"},
			wants:   "memoryRequest must be a memory quantity such as 512Mi or 4Gi",
		},
		{
			name:    "no name",
			request: hardwareTierRequest{ID: "big", CPURequest: "1", CPULimit: "2", MemoryRequest: "1Gi", MemoryLimit: "4Gi"},
			wants:   "name is required: it is what a user picks from",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, err := validatedHardwareTier(testCase.request); err == nil || err.Error() != testCase.wants {
				t.Fatalf("expected %q, got %v", testCase.wants, err)
			}
		})
	}

	valid := hardwareTierRequest{ID: "gpu-ish", Name: "Calcul intensif", CPURequest: "500m", CPULimit: "8", MemoryRequest: "1Gi", MemoryLimit: "32Gi"}
	tier, err := validatedHardwareTier(valid)
	if err != nil {
		t.Fatalf("a sound tier must be accepted: %v", err)
	}
	// Ephemeral storage is optional on the form: an administrator naming a
	// machine should not have to know the phrase "ephemeral storage".
	if tier.EphemeralStorageRequest != "64Mi" || tier.EphemeralStorageLimit != "4Gi" {
		t.Fatalf("ephemeral storage must fall back to a working default, got %+v", tier)
	}
}

func TestTheDefaultTierMovesRatherThanMultiplies(t *testing.T) {
	store := memory.NewHardwareTierStore()
	tier, err := validatedHardwareTier(hardwareTierRequest{ID: "huge", Name: "Énorme", CPURequest: "1", CPULimit: "16", MemoryRequest: "1Gi", MemoryLimit: "64Gi", Default: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Upsert(tier); err != nil {
		t.Fatal(err)
	}
	tiers, err := store.List()
	if err != nil {
		t.Fatal(err)
	}
	defaults := 0
	for _, item := range tiers {
		if item.Default {
			defaults++
		}
	}
	if defaults != 1 {
		t.Fatalf("exactly one tier must be the default, found %d", defaults)
	}
}
