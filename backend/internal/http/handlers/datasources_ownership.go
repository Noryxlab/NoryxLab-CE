package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	datasourcedomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/datasource"
)

// canManageDatasource reads the owner, not the author.
//
// The same distinction every other catalogue object needed the moment it
// became transferable: hand a connector to a team and its author would
// otherwise keep the right to delete it while the owning team would not.
func (h Handlers) canManageDatasource(item datasourcedomain.Datasource, identity auth.Identity) bool {
	if h.isGlobalAdmin(identity) {
		return true
	}
	// Les sujets d'administration : une organisation proprietaire administre
	// son connecteur sans que cela lui ouvre la base.
	for _, subject := range h.ontologyAdminSubjects(identity) {
		if strings.EqualFold(item.OwnerType, subject.Type) && strings.EqualFold(item.OwnerID, subject.ID) {
			return true
		}
	}
	// Les lignes anterieures a la migration n ont pas de proprietaire : leur
	// auteur garde la main plutot que personne.
	return strings.TrimSpace(item.OwnerType) == "" && item.OwnerUserID == identity.UserID()
}

// UpdateDatasourceOwner hands a connector to a person, a team or an
// organization - the last catalogue object that could not be handed on.
//
// A database connector outlives the person who declared it more often than
// anything else here: it points at a system the organization runs, and it
// used to leave with whoever happened to add it.
func (h Handlers) UpdateDatasourceOwner(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasourceStore.GetByID(strings.TrimSpace(r.PathValue("datasourceID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "datasource not found"})
		return
	}
	if !h.canManageDatasource(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "datasource owner or global admin required"})
		return
	}
	var req setDatasetOwnerRequest
	if json.NewDecoder(r.Body).Decode(&req) != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "valid ownerType and ownerId are required"})
		return
	}
	ownerType, ownerID, status, probleme := h.normaliseOwner(req.OwnerType, req.OwnerID, identity)
	if status != 0 {
		writeJSON(w, status, map[string]string{"error": probleme})
		return
	}
	if err := h.datasourceStore.SetOwner(item.ID, ownerType, ownerID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to transfer the datasource"})
		return
	}
	h.settleOwnership(firstNonEmpty(item.OwnerType, "user"), firstNonEmpty(item.OwnerID, item.OwnerUserID), ownerType, ownerID, "writer",
		func(subjectType, subjectID, role string) error {
			now := time.Now().UTC()
			return h.datasourceStore.SetAccess(datasourcedomain.Access{DatasourceID: item.ID, SubjectType: subjectType, SubjectID: subjectID, Role: role, CreatedAt: now, UpdatedAt: now})
		})
	h.emitAudit(r, identity.UserID(), "datasource.owner.changed", "datasource", item.ID, item.Name,
		"success", "", map[string]any{"ownerType": ownerType, "ownerId": ownerID,
			"previousOwnerType": item.OwnerType, "previousOwnerId": item.OwnerID})
	updated, _, _ := h.datasourceStore.GetByID(item.ID)
	writeJSON(w, http.StatusOK, updated)
}

// datasourceRole is what this caller may do with the connector itself.
//
// "owner" for whoever it belongs to as a person or through the owning team,
// then the explicit grant, then nothing. Separate from canManageDatasource for
// the reason the whole catalogue separates them: administering a connector -
// handing it on, deciding who gets it - is not using it against a live
// database.
func (h Handlers) datasourceRole(item datasourcedomain.Datasource, identity auth.Identity) string {
	ownerType, ownerID := item.OwnerType, item.OwnerID
	if strings.TrimSpace(ownerType) == "" {
		ownerType, ownerID = "user", item.OwnerUserID
	}
	for _, subject := range h.ontologySubjects(identity) {
		if strings.EqualFold(ownerType, subject.Type) && strings.EqualFold(ownerID, subject.ID) {
			return "owner"
		}
	}
	if h.datasourceStore == nil {
		return ""
	}
	// Les octrois explicites comptent les organisations, la propriete non.
	//
	// Un octroi est un acte : quelqu'un a decide que telle organisation lit
	// cette donnee, et l'ecran le propose. Il ne se lisait plus - la liste
	// des sujets avait perdu les organisations pour fermer l'heritage de
	// propriete - donc la plateforme acceptait un droit, l'affichait dans la
	// matrice, et ne l'appliquait pas. L'heritage ferme est celui de la
	// propriete ; ce qui est donne nommement s'applique.
	best := ""
	for _, subject := range h.ontologyGrantSubjects(identity) {
		access, found, err := h.datasourceStore.GetAccess(item.ID, subject.Type, subject.ID)
		if err == nil && found && (access.Role == "writer" || best == "") {
			best = access.Role
		}
	}
	return best
}

// canUseDatasource is the data right: open a connection from a workspace, an
// app or a job.
func (h Handlers) canUseDatasource(item datasourcedomain.Datasource, identity auth.Identity) bool {
	return h.datasourceRole(item, identity) != ""
}

// canSeeDatasource is what a catalogue and a permissions screen need.
func (h Handlers) canSeeDatasource(item datasourcedomain.Datasource, identity auth.Identity) bool {
	return h.canUseDatasource(item, identity) || h.canManageDatasource(item, identity)
}

// ListDatasourceAccess shows who may use this connector besides its owner.
func (h Handlers) ListDatasourceAccess(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasourceStore.GetByID(strings.TrimSpace(r.PathValue("datasourceID")))
	if err != nil || !found || !h.canSeeDatasource(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "datasource not found"})
		return
	}
	grants, err := h.datasourceStore.ListAccess(item.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list datasource permissions"})
		return
	}
	ownerType, ownerID := item.OwnerType, item.OwnerID
	if strings.TrimSpace(ownerType) == "" {
		ownerType, ownerID = "user", item.OwnerUserID
	}
	tous := append([]datasourcedomain.Access{{
		DatasourceID: item.ID, SubjectType: ownerType, SubjectID: ownerID,
		Role: "owner", CreatedAt: item.CreatedAt, UpdatedAt: item.UpdatedAt,
	}}, grants...)
	h.nameDatasourceAccessSubjects(tous)
	writeJSON(w, http.StatusOK, map[string]any{
		"items":     tous,
		"canManage": h.canManageDatasource(item, identity),
	})
}

// SetDatasourceAccess grants the use of this connector.
func (h Handlers) SetDatasourceAccess(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasourceStore.GetByID(strings.TrimSpace(r.PathValue("datasourceID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "datasource not found"})
		return
	}
	if !h.canManageDatasource(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "datasource owner or global admin required"})
		return
	}
	subjectType, subjectID, status, probleme := h.grantSubject(r)
	if status != 0 {
		writeJSON(w, status, map[string]string{"error": probleme})
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role == "" {
		role = "reader"
	}
	if role != "reader" && role != "writer" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "role must be reader or writer"})
		return
	}
	if strings.EqualFold(subjectType, item.OwnerType) && strings.EqualFold(subjectID, item.OwnerID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "owner role cannot be changed"})
		return
	}
	now := time.Now().UTC()
	access := datasourcedomain.Access{DatasourceID: item.ID, SubjectType: subjectType, SubjectID: subjectID, Role: role, CreatedAt: now, UpdatedAt: now}
	if existing, exists, _ := h.datasourceStore.GetAccess(item.ID, subjectType, subjectID); exists {
		access.CreatedAt = existing.CreatedAt
	}
	if err := h.datasourceStore.SetAccess(access); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to set datasource permission"})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "datasource.access.grant", "datasource", item.ID, "", "success", "", map[string]any{
		"subjectType": subjectType, "subjectId": subjectID, "role": role, "name": item.Name,
	})
	writeJSON(w, http.StatusOK, access)
}

// DeleteDatasourceAccess takes that use back.
func (h Handlers) DeleteDatasourceAccess(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	item, found, err := h.datasourceStore.GetByID(strings.TrimSpace(r.PathValue("datasourceID")))
	if err != nil || !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "datasource not found"})
		return
	}
	if !h.canManageDatasource(item, identity) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "datasource owner or global admin required"})
		return
	}
	subjectType, subjectID, status, probleme := h.grantSubject(r)
	if status != 0 {
		writeJSON(w, status, map[string]string{"error": probleme})
		return
	}
	if err := h.datasourceStore.DeleteAccess(item.ID, subjectType, subjectID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete datasource permission"})
		return
	}
	h.emitAdvancedAudit(r, identity.UserID(), "datasource.access.revoke", "datasource", item.ID, "", "success", "", map[string]any{
		"subjectType": subjectType, "subjectId": subjectID, "name": item.Name,
	})
	w.WriteHeader(http.StatusNoContent)
}
