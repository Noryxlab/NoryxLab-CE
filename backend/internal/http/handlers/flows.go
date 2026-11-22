package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	flowdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/flow"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/platformrole"
	"github.com/google/uuid"
)

// Les flows vus de l'API.
//
// Declarer un graphe demande le droit de lancer un job, et rien de plus : un
// flow n'est que des jobs enchaines, et exiger un droit a part aurait donne
// une permission qui ne protege rien tout en empechant le geste utile.

type saveFlowRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Steps       []flowdomain.Step `json:"steps"`
	MaxParallel int               `json:"maxParallel"`
}

func (h Handlers) SaveFlow(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	if !h.requireCapability(w, identity, platformrole.LaunchJob) {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if !h.requireProjectRole(w, projectID, identity.UserID(), actionLaunch, "flow declaration") {
		return
	}
	if h.flowStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "flows need the database store"})
		return
	}
	var req saveFlowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	// Le graphe est valide ici, pas a l'execution : un cycle decouvert en
	// cours de route laisse des pods derriere et une personne qui attend,
	// alors que son auteur n'est plus devant le clavier.
	if err := flowdomain.Validate(req.Steps); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a flow needs a name"})
		return
	}
	if req.MaxParallel <= 0 {
		req.MaxParallel = flowdomain.DefaultMaxParallel
	}
	item := flowdomain.Flow{
		ID:          strings.TrimSpace(r.PathValue("flowID")),
		ProjectID:   projectID,
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		OwnerUserID: identity.UserID(),
		Steps:       req.Steps,
		MaxParallel: req.MaxParallel,
		CreatedAt:   time.Now().UTC(),
	}
	if item.ID == "" {
		item.ID = uuid.NewString()
	}
	if err := h.flowStore.SaveFlow(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save the flow"})
		return
	}
	h.emitAudit(r, identity.UserID(), "flow.saved", "flow", item.ID, projectID, "success", "",
		map[string]any{"name": item.Name, "steps": len(item.Steps)})
	writeJSON(w, http.StatusOK, item)
}

func (h Handlers) ListFlows(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if !h.hasProjectMembership(identity.UserID(), projectID) && !h.isGlobalAdmin(identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	if h.flowStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []flowdomain.Flow{}})
		return
	}
	items, err := h.flowStore.ListFlows(projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read flows"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

// StartFlowRun begins an execution, or resumes one.
//
// Resuming carries over what already succeeded rather than re-running it -
// that is the whole point on a graph where one step took forty minutes - and
// records those steps as "reused" rather than "succeeded", so a run never
// claims to have executed something it did not.
func (h Handlers) StartFlowRun(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	if !h.requireCapability(w, identity, platformrole.LaunchJob) {
		return
	}
	if h.flowStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "flows need the database store"})
		return
	}
	definition, found, err := h.flowStore.GetFlow(strings.TrimSpace(r.PathValue("flowID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "flow not found"})
		return
	}
	if !h.requireProjectRole(w, definition.ProjectID, identity.UserID(), actionLaunch, "flow run") {
		return
	}

	resumeOf := strings.TrimSpace(r.URL.Query().Get("resumeOf"))
	reusable := map[string]flowdomain.StepRun{}
	if resumeOf != "" {
		previous, found, err := h.flowStore.GetRun(resumeOf)
		if err != nil || !found || previous.FlowID != definition.ID {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "the run to resume does not belong to this flow"})
			return
		}
		steps, err := h.flowStore.ListStepRuns(resumeOf)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the previous run"})
			return
		}
		for _, step := range steps {
			if step.Status == flowdomain.StepSucceeded || step.Status == flowdomain.StepReused {
				reusable[step.StepKey] = step
			}
		}
	}

	run := flowdomain.Run{
		ID:        uuid.NewString(),
		FlowID:    definition.ID,
		ProjectID: definition.ProjectID,
		Status:    flowdomain.RunRunning,
		StartedBy: identity.UserID(),
		StartedAt: time.Now().UTC(),
		ResumeOf:  resumeOf,
	}
	steps := make([]flowdomain.StepRun, 0, len(definition.Steps))
	for _, step := range definition.Steps {
		entry := flowdomain.StepRun{RunID: run.ID, StepKey: step.Key, Status: flowdomain.StepPending}
		if previous, reused := reusable[step.Key]; reused {
			entry.Status = flowdomain.StepReused
			// Le job d'origine est garde : la provenance d'une etape
			// reutilisee doit pointer sur le run qui l'a reellement produite.
			entry.JobID = previous.JobID
			entry.Reason = "reused from run " + resumeOf
			entry.FinishedAt = previous.FinishedAt
		}
		steps = append(steps, entry)
	}
	if err := h.flowStore.CreateRun(run, steps); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to start the run"})
		return
	}
	h.emitAudit(r, identity.UserID(), "flow.run.started", "flow", definition.ID, definition.ProjectID, "success", "",
		map[string]any{"runId": run.ID, "resumeOf": resumeOf, "reused": len(reusable), "steps": len(steps)})
	// Le premier pas est fait tout de suite plutot qu'au prochain balayage :
	// vingt secondes d'immobilite apres un clic se lisent comme une panne.
	go h.advanceFlowRun(context.Background(), run)
	writeJSON(w, http.StatusCreated, run)
}

func (h Handlers) GetFlowRun(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	if h.flowStore == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "run not found"})
		return
	}
	run, found, err := h.flowStore.GetRun(strings.TrimSpace(r.PathValue("runID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "run not found"})
		return
	}
	if !h.hasProjectMembership(identity.UserID(), run.ProjectID) && !h.isGlobalAdmin(identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "run not found"})
		return
	}
	steps, err := h.flowStore.ListStepRuns(run.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read the run"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"run": run, "steps": steps})
}

func (h Handlers) ListFlowRuns(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	if h.flowStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []flowdomain.Run{}})
		return
	}
	definition, found, err := h.flowStore.GetFlow(strings.TrimSpace(r.PathValue("flowID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "flow not found"})
		return
	}
	if !h.hasProjectMembership(identity.UserID(), definition.ProjectID) && !h.isGlobalAdmin(identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "flow not found"})
		return
	}
	items, err := h.flowStore.ListRuns(definition.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read runs"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}
