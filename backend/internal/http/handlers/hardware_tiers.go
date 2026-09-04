package handlers

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/hardware"
)

// A tier id is part of an API contract and ends up in labels and audit
// entries, so it is restricted to what is safe in all three.
var hardwareTierIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,62}$`)

// Kubernetes CPU ("500m", "2") and memory ("4Gi", "512Mi") quantities. A bad
// quantity is refused here: accepted, it produces a pod the scheduler rejects
// with a message the user never sees.
var (
	cpuQuantityPattern    = regexp.MustCompile(`^([1-9][0-9]*m|[0-9]+(\.[0-9]+)?)$`)
	memoryQuantityPattern = regexp.MustCompile(`^[1-9][0-9]*(Mi|Gi|Ti|M|G|T)$`)
)

func (h Handlers) hardwareTiers() []hardware.Tier {
	if h.hardwareTierStore == nil {
		return hardware.Defaults()
	}
	tiers, err := h.hardwareTierStore.List()
	if err != nil || len(tiers) == 0 {
		// A store that cannot answer must not leave the platform unable to
		// start anything: the compiled defaults still describe real machines.
		return hardware.Defaults()
	}
	return tiers
}

// publicHardwareTier is what a user picking a machine sees. Requests are the
// administrator's business: they are how the cluster is packed, they say
// nothing about what the machine can do, and showing them next to the limit
// invites the question of which number is real.
type publicHardwareTier struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Description           string `json:"description,omitempty"`
	CPULimit              string `json:"cpuLimit"`
	MemoryLimit           string `json:"memoryLimit"`
	EphemeralStorageLimit string `json:"ephemeralStorageLimit"`
	Default               bool   `json:"default"`
}

func (h Handlers) GetHardwareTiers(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireIdentity(w, r); !ok {
		return
	}
	tiers := h.hardwareTiers()
	items := make([]publicHardwareTier, 0, len(tiers))
	for _, tier := range tiers {
		items = append(items, publicHardwareTier{
			ID:                    tier.ID,
			Name:                  tier.Name,
			Description:           tier.Description,
			CPULimit:              tier.CPULimit,
			MemoryLimit:           tier.MemoryLimit,
			EphemeralStorageLimit: tier.EphemeralStorageLimit,
			Default:               tier.Default,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) resolveHardwareTier(raw string) (hardware.Tier, bool) {
	tiers := h.hardwareTiers()
	requested := strings.TrimSpace(raw)
	for _, tier := range tiers {
		if requested != "" && strings.EqualFold(tier.ID, requested) {
			return tier, true
		}
	}
	if requested != "" {
		return hardware.Tier{}, false
	}
	for _, tier := range tiers {
		if tier.Default {
			return tier, true
		}
	}
	if len(tiers) > 0 {
		return tiers[0], true
	}
	return hardware.Tier{}, false
}

type hardwareTierRequest struct {
	ID                      string `json:"id"`
	Name                    string `json:"name"`
	Description             string `json:"description"`
	CPURequest              string `json:"cpuRequest"`
	CPULimit                string `json:"cpuLimit"`
	MemoryRequest           string `json:"memoryRequest"`
	MemoryLimit             string `json:"memoryLimit"`
	EphemeralStorageRequest string `json:"ephemeralStorageRequest"`
	EphemeralStorageLimit   string `json:"ephemeralStorageLimit"`
	Default                 bool   `json:"default"`
	Position                int    `json:"position"`
}

func (h Handlers) ListAdminHardwareTiers(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireGlobalAdmin(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": h.hardwareTiers()})
}

func (h Handlers) SaveHardwareTier(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireGlobalAdmin(w, r)
	if !ok {
		return
	}
	if h.hardwareTierStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "hardware tiers are not editable on this installation"})
		return
	}
	var req hardwareTierRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a valid hardware tier is required"})
		return
	}
	if pathID := strings.TrimSpace(r.PathValue("tierID")); pathID != "" {
		req.ID = pathID
	}
	tier, err := validatedHardwareTier(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := h.hardwareTierStore.Upsert(tier); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save the hardware tier"})
		return
	}
	h.emitAudit(r, identity.UserID(), "hardware-tier.save", "hardware-tier", tier.ID, "", "success", "", map[string]any{
		"name": tier.Name, "cpuLimit": tier.CPULimit, "memoryLimit": tier.MemoryLimit, "default": tier.Default,
	})
	writeJSON(w, http.StatusOK, tier)
}

func (h Handlers) DeleteHardwareTier(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireGlobalAdmin(w, r)
	if !ok {
		return
	}
	if h.hardwareTierStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "hardware tiers are not editable on this installation"})
		return
	}
	id := strings.TrimSpace(r.PathValue("tierID"))
	tiers := h.hardwareTiers()
	// Refusing to remove the last tier, and refusing to remove the default
	// without naming a new one: either would leave the platform unable to
	// start a workspace, and the administrator would find out from a user.
	if len(tiers) <= 1 {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "the last hardware tier cannot be removed"})
		return
	}
	for _, tier := range tiers {
		if strings.EqualFold(tier.ID, id) && tier.Default {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "the default tier cannot be removed: make another tier the default first"})
			return
		}
	}
	if err := h.hardwareTierStore.Delete(id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to remove the hardware tier"})
		return
	}
	h.emitAudit(r, identity.UserID(), "hardware-tier.delete", "hardware-tier", id, "", "success", "", nil)
	w.WriteHeader(http.StatusNoContent)
}

func validatedHardwareTier(req hardwareTierRequest) (hardware.Tier, error) {
	tier := hardware.Tier{
		ID:                      strings.TrimSpace(req.ID),
		Name:                    strings.TrimSpace(req.Name),
		Description:             strings.TrimSpace(req.Description),
		CPURequest:              strings.TrimSpace(req.CPURequest),
		CPULimit:                strings.TrimSpace(req.CPULimit),
		MemoryRequest:           strings.TrimSpace(req.MemoryRequest),
		MemoryLimit:             strings.TrimSpace(req.MemoryLimit),
		EphemeralStorageRequest: strings.TrimSpace(req.EphemeralStorageRequest),
		EphemeralStorageLimit:   strings.TrimSpace(req.EphemeralStorageLimit),
		Default:                 req.Default,
		Position:                req.Position,
	}
	if !hardwareTierIDPattern.MatchString(tier.ID) {
		return hardware.Tier{}, errInvalidHardwareTier("id must be 1 to 63 letters, digits, dot, dash or underscore")
	}
	if tier.Name == "" {
		return hardware.Tier{}, errInvalidHardwareTier("name is required: it is what a user picks from")
	}
	if tier.EphemeralStorageRequest == "" {
		tier.EphemeralStorageRequest = "64Mi"
	}
	if tier.EphemeralStorageLimit == "" {
		tier.EphemeralStorageLimit = "4Gi"
	}
	for label, value := range map[string]string{"cpuRequest": tier.CPURequest, "cpuLimit": tier.CPULimit} {
		if !cpuQuantityPattern.MatchString(value) {
			return hardware.Tier{}, errInvalidHardwareTier(label + " must be a CPU quantity such as 500m or 2")
		}
	}
	for label, value := range map[string]string{
		"memoryRequest":           tier.MemoryRequest,
		"memoryLimit":             tier.MemoryLimit,
		"ephemeralStorageRequest": tier.EphemeralStorageRequest,
		"ephemeralStorageLimit":   tier.EphemeralStorageLimit,
	} {
		if !memoryQuantityPattern.MatchString(value) {
			return hardware.Tier{}, errInvalidHardwareTier(label + " must be a memory quantity such as 512Mi or 4Gi")
		}
	}
	// A request above its limit is refused by Kubernetes at admission, long
	// after the administrator has left the screen believing the tier is saved.
	if cpuCores(tier.CPURequest) > cpuCores(tier.CPULimit) {
		return hardware.Tier{}, errInvalidHardwareTier("cpuRequest cannot exceed cpuLimit")
	}
	if memoryBytes(tier.MemoryRequest) > memoryBytes(tier.MemoryLimit) {
		return hardware.Tier{}, errInvalidHardwareTier("memoryRequest cannot exceed memoryLimit")
	}
	return tier, nil
}

type hardwareTierError string

func (e hardwareTierError) Error() string { return string(e) }

func errInvalidHardwareTier(message string) error { return hardwareTierError(message) }

func parseFloat(raw string) float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0
	}
	return value
}

func cpuCores(quantity string) float64 {
	if strings.HasSuffix(quantity, "m") {
		return parseFloat(strings.TrimSuffix(quantity, "m")) / 1000
	}
	return parseFloat(quantity)
}

func memoryBytes(quantity string) float64 {
	units := []struct {
		suffix string
		factor float64
	}{
		{"Ti", 1 << 40}, {"Gi", 1 << 30}, {"Mi", 1 << 20},
		{"T", 1e12}, {"G", 1e9}, {"M", 1e6},
	}
	for _, unit := range units {
		if strings.HasSuffix(quantity, unit.suffix) {
			return parseFloat(strings.TrimSuffix(quantity, unit.suffix)) * unit.factor
		}
	}
	return parseFloat(quantity)
}
