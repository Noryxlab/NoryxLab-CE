package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/platformrole"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/pod"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

type launchPodRequest struct {
	ProjectID string            `json:"projectId"`
	PodName   string            `json:"podName"`
	Image     string            `json:"image"`
	Command   []string          `json:"command"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
}

// ListPods answers with the launches of the projects this caller belongs to.
//
// It asked nobody anything. The request was discarded in the signature, and
// neither middleware authenticates - one audits mutations, the other narrows a
// token that is already there. So an unauthenticated GET returned every pod
// launch on the installation: project identifiers, images, commands, and the
// environment each was given, which is where somebody puts a credential when
// they are in a hurry. Verified open on the production cluster on 2026-10-26,
// with an empty table, which is the only reason this is a near miss rather
// than an incident.
func (h Handlers) ListPods(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	items, err := h.podStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list pod launches"})
		return
	}
	userID := identity.UserID()
	visibles := make([]pod.Launch, 0, len(items))
	for _, item := range items {
		if h.isGlobalAdmin(identity) || h.hasProjectMembership(userID, item.ProjectID) {
			visibles = append(visibles, item)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": visibles})
}

func (h Handlers) LaunchPod(w http.ResponseWriter, r *http.Request) {
	identity, userID, ok := h.requireUserIdentity(w, r)
	if !ok {
		return
	}
	// La matrice decide (ADR-049) : un pod est une charge de calcul.
	if !h.requireCapability(w, identity, platformrole.LaunchJob) {
		return
	}

	var req launchPodRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}

	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.Image = strings.TrimSpace(req.Image)
	req.PodName = strings.TrimSpace(req.PodName)
	if req.ProjectID == "" || req.Image == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectId and image are required"})
		return
	}
	if req.PodName == "" {
		req.PodName = "run-" + shortID()
	}

	exists, err := h.projectExists(req.ProjectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to verify project"})
		return
	}
	if !exists {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	if !h.requireProjectRole(w, req.ProjectID, userID, actionLaunch, "pod launch") {
		return
	}

	record := pod.New(req.ProjectID, req.PodName, req.Image, req.Command, req.Args, req.Env)

	if h.runtime != nil {
		env := make([]noryxruntime.EnvVar, 0, len(req.Env))
		for k, v := range req.Env {
			env = append(env, noryxruntime.EnvVar{Name: k, Value: v})
		}

		err = h.runtime.CreatePod(noryxruntime.PodSpec{
			PodName:    req.PodName,
			Image:      req.Image,
			Command:    req.Command,
			Args:       req.Args,
			Env:        env,
			PullSecret: h.registryPullSecret,
			Labels: map[string]string{
				"app.kubernetes.io/name": "noryx-pod",
				"noryx.io/project-id":    req.ProjectID,
				"noryx.io/pod-id":        record.ID,
			},
		})
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "kubernetes pod launch failed: " + err.Error()})
			return
		}
	}

	if err := h.podStore.Create(record); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to save pod launch"})
		return
	}

	h.emitAudit(r, userID, "pod.launch", "pod", record.ID, record.ProjectID, "success", "", map[string]any{
		"podName": record.PodName,
		"image":   record.Image,
	})
	writeJSON(w, http.StatusCreated, record)
}
