package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/datasource"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	_ "github.com/lib/pq"
)

type createDatasourceRequest struct {
	Name           string `json:"name"`
	Type           string `json:"type"`
	Host           string `json:"host"`
	Port           int    `json:"port"`
	Database       string `json:"database"`
	Username       string `json:"username"`
	PasswordSecret string `json:"passwordSecret"`
	SSLMode        string `json:"sslMode"`
}

func (h Handlers) ListDatasources(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	items, err := h.datasourceStore.ListByUser(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list datasources"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) CreateDatasource(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	var req createDatasourceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Type = strings.ToLower(strings.TrimSpace(req.Type))
	req.Host = strings.TrimSpace(req.Host)
	req.Database = strings.TrimSpace(req.Database)
	req.Username = strings.TrimSpace(req.Username)
	req.PasswordSecret = strings.TrimSpace(req.PasswordSecret)
	req.SSLMode = strings.TrimSpace(req.SSLMode)
	if req.Name == "" || req.Type == "" || req.Host == "" || req.Database == "" || req.Username == "" || req.PasswordSecret == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name,type,host,database,username,passwordSecret are required"})
		return
	}
	if req.Type != "postgres" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "supported datasource type in V1: postgres"})
		return
	}
	if req.Port <= 0 {
		req.Port = 5432
	}
	if req.SSLMode == "" {
		req.SSLMode = "disable"
	}
	item := datasource.New(userID, req.Name, req.Type, req.Host, req.Database, req.Username, req.PasswordSecret, req.SSLMode, req.Port)
	if err := h.datasourceStore.Create(item); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create datasource"})
		return
	}
	writeJSON(w, http.StatusCreated, item)
}

func (h Handlers) DeleteDatasource(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	datasourceID := strings.TrimSpace(r.PathValue("datasourceID"))
	if datasourceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "datasourceID is required"})
		return
	}
	item, found, err := h.datasourceStore.GetByID(datasourceID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read datasource"})
		return
	}
	if !found || item.OwnerUserID != userID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "datasource not found"})
		return
	}
	if err := h.datasourceStore.Delete(datasourceID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete datasource"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) ValidateDatasource(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	datasourceID := strings.TrimSpace(r.PathValue("datasourceID"))
	item, found, err := h.datasourceStore.GetByID(datasourceID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read datasource"})
		return
	}
	if !found || item.OwnerUserID != userID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "datasource not found"})
		return
	}
	if item.Type != "postgres" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "validate supports postgres in V1"})
		return
	}
	password, err := h.resolveRepositorySecretValueForWorkspace(userID, item.PasswordSecret)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read datasource secret: " + err.Error()})
		return
	}
	dsn := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s connect_timeout=4", item.Host, item.Port, item.Database, item.Username, password, item.SSLMode)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid datasource configuration"})
		return
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	err = db.PingContext(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"reachable": false, "error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"reachable": true})
}

func (h Handlers) ListProjectDatasources(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if projectID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID is required"})
		return
	}
	if !h.requireProjectMember(w, projectID, userID, "datasource listing") {
		return
	}
	ids, err := h.projectResourceStore.ListProjectDatasourceIDs(projectID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list project datasources"})
		return
	}
	items := make([]datasource.Datasource, 0, len(ids))
	for _, id := range ids {
		item, found, err := h.datasourceStore.GetByID(id)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to load project datasource"})
			return
		}
		if found {
			items = append(items, item)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) AttachProjectDatasource(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	datasourceID := strings.TrimSpace(r.PathValue("datasourceID"))
	if projectID == "" || datasourceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID and datasourceID are required"})
		return
	}
	if !h.requireProjectRole(w, projectID, userID, access.Role.CanLaunchPod, "datasource attach") {
		return
	}
	item, found, err := h.datasourceStore.GetByID(datasourceID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read datasource"})
		return
	}
	if !found || item.OwnerUserID != userID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "datasource not found"})
		return
	}
	if err := h.projectResourceStore.AttachDatasource(projectID, datasourceID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to attach datasource"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h Handlers) DetachProjectDatasource(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	datasourceID := strings.TrimSpace(r.PathValue("datasourceID"))
	if projectID == "" || datasourceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID and datasourceID are required"})
		return
	}
	if !h.requireProjectRole(w, projectID, userID, access.Role.CanLaunchPod, "datasource detach") {
		return
	}
	if err := h.projectResourceStore.DetachDatasource(projectID, datasourceID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to detach datasource"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func datasourceEnvPrefix(name string) string {
	base := strings.ToUpper(strings.TrimSpace(name))
	repl := strings.NewReplacer("-", "_", " ", "_", ".", "_", "/", "_")
	base = repl.Replace(base)
	base = strings.Map(func(r rune) rune {
		switch {
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '_':
			return r
		default:
			return -1
		}
	}, base)
	base = strings.Trim(base, "_")
	if base == "" {
		base = "DS"
	}
	return "NORYX_DS_" + base
}

func (h Handlers) resolveProjectDatasourceEnv(projectID, userID string) ([]noryxruntime.EnvVar, error) {
	ids, err := h.projectResourceStore.ListProjectDatasourceIDs(projectID)
	if err != nil {
		return nil, err
	}
	out := []noryxruntime.EnvVar{}
	for _, id := range ids {
		item, found, err := h.datasourceStore.GetByID(id)
		if err != nil || !found || item.OwnerUserID != userID {
			continue
		}
		password, err := h.resolveRepositorySecretValueForWorkspace(userID, item.PasswordSecret)
		if err != nil {
			continue
		}
		prefix := datasourceEnvPrefix(item.Name)
		out = append(out,
			noryxruntime.EnvVar{Name: prefix + "_TYPE", Value: item.Type},
			noryxruntime.EnvVar{Name: prefix + "_HOST", Value: item.Host},
			noryxruntime.EnvVar{Name: prefix + "_PORT", Value: fmt.Sprintf("%d", item.Port)},
			noryxruntime.EnvVar{Name: prefix + "_DATABASE", Value: item.Database},
			noryxruntime.EnvVar{Name: prefix + "_USERNAME", Value: item.Username},
			noryxruntime.EnvVar{Name: prefix + "_PASSWORD", Value: password},
			noryxruntime.EnvVar{Name: prefix + "_SSLMODE", Value: item.SSLMode},
		)
	}
	return out, nil
}
