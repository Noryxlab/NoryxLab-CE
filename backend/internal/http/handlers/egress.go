package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/egress"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/edition"
)

type createEgressRuleRequest struct {
	ProjectID     string     `json:"projectId"`
	SubjectType   string     `json:"subjectType"`
	SubjectID     string     `json:"subjectId"`
	Profile       string     `json:"profile"`
	Destination   string     `json:"destination"`
	Port          int        `json:"port"`
	Protocol      string     `json:"protocol"`
	WorkloadTypes []string   `json:"workloadTypes"`
	Justification string     `json:"justification"`
	ExpiresAt     *time.Time `json:"expiresAt"`
}

type decideEgressRuleRequest struct {
	Status string `json:"status"`
	Note   string `json:"note"`
}

func (h Handlers) ListEgressProfiles(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireUserID(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items":       egress.Profiles(),
		"enforced":    false,
		"editionOnly": true,
	})
}

func (h Handlers) ListProjectEgressRules(w http.ResponseWriter, r *http.Request) {
	if !h.requireControlledEgress(w) {
		return
	}
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if !h.requireProjectMember(w, projectID, identity.UserID(), "egress rule listing") {
		return
	}
	items, err := h.egressRuleStore.ListByProject(projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list egress rules"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "enforced": false})
}

func (h Handlers) CreateEgressRule(w http.ResponseWriter, r *http.Request) {
	if !h.requireControlledEgress(w) {
		return
	}
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	var req createEgressRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	req.ProjectID = strings.TrimSpace(req.ProjectID)
	req.SubjectType = strings.ToLower(strings.TrimSpace(req.SubjectType))
	req.SubjectID = strings.TrimSpace(req.SubjectID)
	req.Profile = strings.TrimSpace(req.Profile)
	req.Destination = strings.TrimSpace(req.Destination)
	req.Protocol = strings.TrimSpace(req.Protocol)
	req.Justification = strings.TrimSpace(req.Justification)
	if req.ProjectID == "" || req.Destination == "" || req.Justification == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectId, destination and justification are required"})
		return
	}
	if req.Port <= 0 || req.Port > 65535 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "port must be between 1 and 65535"})
		return
	}
	if !egress.ValidProfile(req.Profile) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid egress profile"})
		return
	}
	if strings.EqualFold(req.Profile, "unrestricted") && !h.isGlobalAdmin(identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "unrestricted egress is administrator-only"})
		return
	}
	if !h.requireProjectMember(w, req.ProjectID, identity.UserID(), "egress request") {
		return
	}
	if req.SubjectType == "" {
		req.SubjectType = "user"
	}
	if req.SubjectID == "" {
		req.SubjectID = identity.UserID()
	}
	if !h.canManageEgressSubject(identity, req.SubjectType, req.SubjectID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient rights for egress subject"})
		return
	}
	item := egress.New(req.ProjectID, identity.UserID(), req.SubjectType, req.SubjectID, req.Profile, req.Destination, req.Port, req.Protocol, req.WorkloadTypes, req.Justification, req.ExpiresAt)
	if err := h.egressRuleStore.Create(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create egress rule"})
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h Handlers) ListAdminEgressRules(w http.ResponseWriter, r *http.Request) {
	if !h.requireControlledEgress(w) {
		return
	}
	if _, ok := h.requireAdminModule(w, r, "egress"); !ok {
		return
	}
	items, err := h.egressRuleStore.List()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list egress rules"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "profiles": egress.Profiles(), "enforced": false})
}

func (h Handlers) DecideAdminEgressRule(w http.ResponseWriter, r *http.Request) {
	if !h.requireControlledEgress(w) {
		return
	}
	identity, ok := h.requireAdminModule(w, r, "egress")
	if !ok {
		return
	}
	id := strings.TrimSpace(r.PathValue("egressRuleID"))
	var req decideEgressRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	status := strings.ToLower(strings.TrimSpace(req.Status))
	if status != "approved" && status != "rejected" && status != "revoked" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "status must be approved, rejected or revoked"})
		return
	}
	item, found, err := h.egressRuleStore.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load egress rule"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "egress rule not found"})
		return
	}
	if err := h.egressRuleStore.UpdateDecision(id, status, identity.UserID(), req.Note); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update egress rule"})
		return
	}
	h.emitAudit(r, identity.UserID(), "egress.rule."+status, "egress_rule", id, item.ProjectID, "success", "", map[string]any{"destination": item.Destination, "profile": item.Profile})
	updated, _, _ := h.egressRuleStore.GetByID(id)
	writeJSON(w, http.StatusOK, updated)
}

func (h Handlers) requireControlledEgress(w http.ResponseWriter) bool {
	if h.featureEnabled(edition.FeatureControlledEgress) {
		return true
	}
	writeJSON(w, http.StatusForbidden, map[string]string{"error": "controlled egress is an Enterprise feature"})
	return false
}

func (h Handlers) canManageEgressSubject(identity auth.Identity, subjectType, subjectID string) bool {
	subjectType = strings.ToLower(strings.TrimSpace(subjectType))
	subjectID = strings.TrimSpace(subjectID)
	if subjectID == "" {
		return false
	}
	if h.isGlobalAdmin(identity) {
		return true
	}
	if subjectType == "organization" {
		return h.userBelongsToOrganization(identity.UserID(), subjectID)
	}
	return subjectType == "user" && strings.EqualFold(subjectID, identity.UserID())
}
