package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"time"
)

// What the platform's AI services can do right now.
//
// The assistant, the code assistant and - next - the agents all answer from the
// same model gateway, so they share one state: everything reachable, some of it,
// or none. A person opening the platform should see that before discovering it
// from an assistant that will not answer.
//
// The colours are the point. Green is full service. Amber means a reduced model
// is standing in: basic questions still work, code assistance and agents do not,
// and saying so is better than letting a small model answer a hard question
// confidently and wrongly. Red means nothing is serving.

type aiServicesStatus struct {
	// Configured is false where no gateway is deployed. An installation without
	// AI services must show nothing at all, not a fault for something that was
	// never installed.
	Configured bool `json:"configured"`
	// Agents says whether this installation can run agents at all, which is a
	// property of what is deployed rather than of what a model can do today.
	// It is answered even when no gateway is configured, because an interface
	// deciding whether to offer the section needs it either way.
	Agents       bool            `json:"agents"`
	Mode         string          `json:"mode,omitempty"`
	Capabilities map[string]bool `json:"capabilities,omitempty"`
	// Detail explains a mode that is not full, for whoever can act on it.
	Detail    string    `json:"detail,omitempty"`
	CheckedAt time.Time `json:"checkedAt,omitempty"`
}

// aiServicesCache keeps the last answer briefly.
//
// Every page load would otherwise probe the gateway, which probes its own
// backends: a home page opened by forty people would turn one question into
// forty round trips to a rented GPU.
type aiServicesCache struct {
	mu        sync.Mutex
	status    aiServicesStatus
	fetchedAt time.Time
}

var aiServices aiServicesCache

const aiServicesCacheFor = 15 * time.Second

func (h Handlers) GetAIServicesStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireIdentityFromSessionOrBearer(w, r); !ok {
		return
	}
	if strings.TrimSpace(h.llmaasBaseURL) == "" {
		writeJSON(w, http.StatusOK, aiServicesStatus{Configured: false, Agents: h.agentsDeployed()})
		return
	}

	aiServices.mu.Lock()
	defer aiServices.mu.Unlock()
	if time.Since(aiServices.fetchedAt) < aiServicesCacheFor && aiServices.status.Configured {
		writeJSON(w, http.StatusOK, aiServices.status)
		return
	}

	status := h.readAIServicesStatus(r)
	status.Agents = h.agentsDeployed()
	aiServices.status = status
	aiServices.fetchedAt = time.Now()
	writeJSON(w, http.StatusOK, status)
}

func (h Handlers) readAIServicesStatus(r *http.Request) aiServicesStatus {
	status := aiServicesStatus{Configured: true, CheckedAt: time.Now().UTC()}

	request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, h.llmaasBaseURL+"/v1/status", nil)
	if err != nil {
		status.Mode = "down"
		status.Detail = err.Error()
		return status
	}
	if h.llmaasAPIKey != "" {
		request.Header.Set("Authorization", "Bearer "+h.llmaasAPIKey)
	}
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		// A gateway that cannot be reached is down as far as anybody using the
		// platform is concerned, whatever the reason on its side.
		status.Mode = "down"
		status.Detail = "the model gateway is unreachable"
		return status
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		status.Mode = "down"
		status.Detail = "the model gateway answered " + response.Status
		return status
	}

	var payload struct {
		Mode         string          `json:"mode"`
		Capabilities map[string]bool `json:"capabilities"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		status.Mode = "down"
		status.Detail = "the model gateway answered something unreadable"
		return status
	}
	status.Mode = payload.Mode
	status.Capabilities = payload.Capabilities
	if payload.Mode == "degraded" {
		status.Detail = "a reduced model is standing in: basic questions work, code assistance and agents do not"
	}
	return status
}

// agentsDeployed reports whether this installation has the parts an agent
// needs: somewhere to remember it, and a runner to do the work.
//
// Not the edition, and not a feature flag. A platform whose assistant service
// has been stood down - because it had no model to talk to - still calls
// itself Enterprise, and offering a section where every run fails is the
// "broken rather than absent" mistake the AI services card exists to avoid.
func (h Handlers) agentsDeployed() bool {
	return h.agentStore != nil && strings.TrimSpace(h.assistantURL) != ""
}
