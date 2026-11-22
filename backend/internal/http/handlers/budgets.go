package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	budgetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/budget"
)

// Les budgets vus de l'API.
//
// Poser un budget est un acte d'administration : c'est arbitrer entre projets
// sur une ressource partagee, ce qu'un membre de projet ne peut pas faire pour
// les autres. Lire le sien, en revanche, demande d'etre dans le projet et rien
// de plus - une equipe qui ne voit pas sa consommation ne peut pas la changer.

type setBudgetRequest struct {
	VCPUHours float64 `json:"vcpuHours"`
}

// ListBudgets answers with every project, its allowance and what it has used
// this month - including the projects with no allowance at all.
func (h Handlers) ListBudgets(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	if !h.isGlobalAdmin(identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "administrator required"})
		return
	}
	if h.budgetStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []budgetdomain.State{}})
		return
	}
	states, err := h.budgetStore.ListBudgetStates(monthStart(time.Now()))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read budgets"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": states, "unit": "vcpu-hours", "period": monthStart(time.Now())})
}

// SetBudget sets or clears one project's monthly allowance.
func (h Handlers) SetBudget(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	if !h.isGlobalAdmin(identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "administrator required"})
		return
	}
	if h.budgetStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "budgets need the database store"})
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if exists, err := h.projectExists(projectID); err != nil || !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	var req setBudgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON body"})
		return
	}
	// Un budget negatif n'a pas de sens, et zero en a un : c'est le retrait.
	// Les distinguer evite qu'une faute de frappe devienne une limite absurde
	// qui alerte sans arret.
	if req.VCPUHours < 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "vcpuHours must be zero (no budget) or positive"})
		return
	}
	if req.VCPUHours == 0 {
		if err := h.budgetStore.DeleteProjectBudget(projectID); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to clear budget"})
			return
		}
		h.emitAudit(r, identity.UserID(), "project.budget.cleared", "project", projectID, projectID, "success", "", nil)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if err := h.budgetStore.SetProjectBudget(budgetdomain.Budget{
		ProjectID: projectID,
		VCPUHours: req.VCPUHours,
		UpdatedBy: identity.UserID(),
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to set budget"})
		return
	}
	h.emitAudit(r, identity.UserID(), "project.budget.set", "project", projectID, projectID, "success", "",
		map[string]any{"vcpuHours": req.VCPUHours})
	w.WriteHeader(http.StatusNoContent)
}

// GetProjectBudget answers with one project's own allowance and consumption,
// for the people who work in it.
func (h Handlers) GetProjectBudget(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if !h.hasProjectMembership(identity.UserID(), projectID) && !h.isGlobalAdmin(identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	if h.budgetStore == nil {
		writeJSON(w, http.StatusOK, budgetdomain.State{ProjectID: projectID})
		return
	}
	states, err := h.budgetStore.ListBudgetStates(monthStart(time.Now()))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read budget"})
		return
	}
	for _, state := range states {
		if state.ProjectID == projectID {
			writeJSON(w, http.StatusOK, state)
			return
		}
	}
	writeJSON(w, http.StatusOK, budgetdomain.State{ProjectID: projectID, Period: monthStart(time.Now())})
}
