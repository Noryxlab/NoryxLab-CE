package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/projectvar"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/security"
)

// A project's environment variables.
//
// A secret belongs to a person; a variable belongs to the work. The tracking
// server's address, the bucket, the model registry's URL are the same for
// everybody on the project, and holding them as personal secrets meant a
// colleague could not run what you ran until they had recreated your secrets
// by hand, under the same names, from a list nobody had written down.
//
// Who may see what follows the role, and it is not the same question as who
// may run:
//
//   - a viewer sees the names. They tell you what a workload expects, and a
//     name is not a value.
//   - an editor and an admin see the values, and set them. They are also the
//     roles that can launch, so they receive the values in a shell anyway -
//     hiding them on the screen would be theatre.
//
// Values are encrypted at rest exactly as a secret is. The screen says these
// are not for credentials; people will put a connection string in one anyway,
// and a database dump should not be the place they find out it mattered.

type projectVariableView struct {
	Name        string    `json:"name"`
	Value       string    `json:"value,omitempty"`
	Description string    `json:"description,omitempty"`
	UpdatedBy   string    `json:"updatedBy,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type upsertProjectVariableRequest struct {
	Value       string `json:"value"`
	Description string `json:"description"`
}

func (h Handlers) ListProjectVariables(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if !h.hasProjectMembership(userID, projectID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "project membership required"})
		return
	}
	if h.projectVariableStore == nil {
		writeJSON(w, http.StatusOK, map[string]any{"items": []projectVariableView{}, "canReadValues": false})
		return
	}
	items, err := h.projectVariableStore.ListByProject(projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list project variables"})
		return
	}

	canReadValues := h.allowsProjectAction(projectID, userID, actionLaunch)
	out := make([]projectVariableView, 0, len(items))
	for _, item := range items {
		view := projectVariableView{
			Name:        item.Name,
			Description: item.Description,
			UpdatedBy:   item.UpdatedBy,
			UpdatedAt:   item.UpdatedAt,
		}
		if canReadValues {
			value, err := security.DecryptString(h.secretsMasterKey, item.ValueEncrypted)
			if err != nil {
				// One unreadable value must not hide the rest: the name is
				// still worth showing, and the empty value says something is
				// wrong with that one row rather than with the screen.
				view.Value = ""
			} else {
				view.Value = value
			}
		}
		out = append(out, view)
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out, "canReadValues": canReadValues})
}

func (h Handlers) UpsertProjectVariable(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if !h.requireProjectRole(w, projectID, userID, actionLaunch, "project variables") {
		return
	}
	if h.projectVariableStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "project variables are not configured"})
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	if err := projectvar.ValidateName(name); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	var req upsertProjectVariableRequest
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a value is required"})
		return
	}
	if len(req.Value) > projectvar.MaxValueLength {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "a value is at most 8192 characters"})
		return
	}
	if strings.TrimSpace(h.secretsMasterKey) == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "secrets encryption key is not configured"})
		return
	}
	encrypted, err := security.EncryptString(h.secretsMasterKey, req.Value)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store the variable"})
		return
	}

	item := projectvar.New(projectID, name, encrypted, req.Description, userID)
	if existing, found, _ := h.projectVariableStore.GetByName(projectID, name); found {
		item.CreatedAt = existing.CreatedAt
	}
	if err := h.projectVariableStore.Upsert(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to store the variable"})
		return
	}
	// The value is never in the audit trail: the point of encrypting it at
	// rest would be lost if it were written in clear next to who set it.
	h.emitAudit(r, userID, "project.variable.set", "project_variable", name, projectID, "success", "", map[string]any{
		"description": item.Description,
	})
	writeJSON(w, http.StatusOK, projectVariableView{
		Name:        item.Name,
		Value:       req.Value,
		Description: item.Description,
		UpdatedBy:   item.UpdatedBy,
		UpdatedAt:   item.UpdatedAt,
	})
}

func (h Handlers) DeleteProjectVariable(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if !h.requireProjectRole(w, projectID, userID, actionLaunch, "project variables") {
		return
	}
	if h.projectVariableStore == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "project variables are not configured"})
		return
	}
	name := strings.TrimSpace(r.PathValue("name"))
	if _, found, _ := h.projectVariableStore.GetByName(projectID, name); !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no variable named " + name})
		return
	}
	if err := h.projectVariableStore.Delete(projectID, name); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete the variable"})
		return
	}
	h.emitAudit(r, userID, "project.variable.delete", "project_variable", name, projectID, "success", "", nil)
	w.WriteHeader(http.StatusNoContent)
}

// projectVariableEnv is what a workload receives: the project's variables,
// under their own names. Not prefixed, because a variable exists to be read by
// the tool that expects it - MLFLOW_TRACKING_URI is the name mlflow looks for,
// and NORYX_VAR_MLFLOW_TRACKING_URI is a name nothing looks for.
func (h Handlers) projectVariableEnv(projectID string) map[string]string {
	if h.projectVariableStore == nil || strings.TrimSpace(projectID) == "" {
		return nil
	}
	items, err := h.projectVariableStore.ListByProject(projectID)
	if err != nil || len(items) == 0 {
		return nil
	}
	out := make(map[string]string, len(items))
	for _, item := range items {
		value, err := security.DecryptString(h.secretsMasterKey, item.ValueEncrypted)
		if err != nil {
			continue
		}
		out[item.Name] = value
	}
	return out
}

// workloadEnvData is what a workload's environment secret holds: the launching
// person's secrets, under NORYX_SECRET_…, and the project's variables under
// their own names. The two cannot collide - a variable may not begin with
// NORYX_ - so the merge is safe by construction rather than by luck.
func (h Handlers) workloadEnvData(projectID, userID string) (map[string]string, error) {
	data, err := h.resolveUserSecretEnv(userID)
	if err != nil {
		return nil, err
	}
	if data == nil {
		data = map[string]string{}
	}
	for name, value := range h.projectVariableEnv(projectID) {
		data[name] = value
	}
	return data, nil
}
