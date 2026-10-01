package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

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
	for _, subject := range h.ontologySubjects(identity) {
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
	h.emitAudit(r, identity.UserID(), "datasource.owner.changed", "datasource", item.ID, item.Name,
		"success", "", map[string]any{"ownerType": ownerType, "ownerId": ownerID,
			"previousOwnerType": item.OwnerType, "previousOwnerId": item.OwnerID})
	updated, _, _ := h.datasourceStore.GetByID(item.ID)
	writeJSON(w, http.StatusOK, updated)
}
