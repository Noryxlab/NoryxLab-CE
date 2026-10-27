package handlers

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/platformrole"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/datasource"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/hardware"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/secret"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/security"
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

type createDataServiceRequest struct {
	Name         string `json:"name"`
	DefinitionID string `json:"definitionId"`
	Database     string `json:"database"`
	Username     string `json:"username"`
	StorageSize  string `json:"storageSize"`
	HardwareTier string `json:"hardwareTier"`
}

func (h Handlers) ListDatasources(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	userID := identity.UserID()
	items, err := h.datasourceStore.ListByUser(userID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list datasources"})
		return
	}
	// Ce qu'on nous a prete et ce qu'on administre, a cote de ce qu'on a
	// declare soi-meme : la liste ne connaissait que l'auteur, donc un
	// connecteur transfere disparaissait de chez son nouveau proprietaire.
	vus := map[string]bool{}
	for _, item := range items {
		vus[item.ID] = true
	}
	for _, subject := range h.ontologyAdminSubjects(identity) {
		pretes, err := h.datasourceStore.ListGrantedTo(subject.Type, subject.ID)
		if err != nil {
			continue
		}
		for _, item := range pretes {
			if !vus[item.ID] {
				vus[item.ID] = true
				items = append(items, item)
			}
		}
	}
	if tous, err := h.datasourceStore.ListAll(); err == nil {
		for _, item := range tous {
			if !vus[item.ID] && h.canSeeDatasource(item, identity) {
				vus[item.ID] = true
				items = append(items, item)
			}
		}
	}
	for i := range items {
		items[i] = h.enrichDatasourceStatus(items[i])
	}
	h.nameDatasourceOwners(items)
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) ListDatasourceDefinitions(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireUserID(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": datasource.SystemServiceDefinitions()})
}

func (h Handlers) CreateDataService(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	// La matrice decide (ADR-049).
	if !h.requireCapability(w, auth.Identity{Username: userID, Subject: userID}, platformrole.DeclareDatasource) {
		return
	}
	if h.runtime == nil {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "kubernetes runtime is disabled"})
		return
	}
	var req createDataServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON payload"})
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.DefinitionID = strings.TrimSpace(req.DefinitionID)
	req.Database = strings.TrimSpace(req.Database)
	req.Username = strings.TrimSpace(req.Username)
	req.StorageSize = strings.TrimSpace(req.StorageSize)
	req.HardwareTier = strings.TrimSpace(req.HardwareTier)
	if req.Name == "" || req.DefinitionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name and definitionId are required"})
		return
	}
	definition, found := datasourceDefinitionByID(req.DefinitionID)
	if !found {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown data service definition"})
		return
	}
	if req.Database == "" {
		req.Database = "noryx"
	}
	if req.Username == "" {
		req.Username = "noryx"
	}
	if req.StorageSize == "" {
		req.StorageSize = "10Gi"
	}
	if !validStorageSize(req.StorageSize) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "storageSize must be a Kubernetes quantity such as 10Gi"})
		return
	}
	tier, found := h.resolveHardwareTier(req.HardwareTier)
	if !found {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown hardwareTier"})
		return
	}
	if strings.TrimSpace(h.secretsMasterKey) == "" {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "secrets encryption key is not configured"})
		return
	}
	password, err := generatedDatasourcePassword()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to generate credentials"})
		return
	}
	resourceName := "dataservice-" + shortID()
	pvcName := resourceName + "-data"
	kubeSecretName := resourceName + "-credentials"
	noryxSecretName := resourceName + "-password"
	labels := map[string]string{
		"app.kubernetes.io/name":      "noryx-data-service",
		"noryx.io/data-service":       resourceName,
		"noryx.io/data-service-type":  definition.Type,
		"noryx.io/data-service-owner": sanitizeK8sName(userID),
	}
	cleanup := func() {
		_ = h.runtime.DeleteService(resourceName)
		_ = h.runtime.DeletePod(resourceName)
		_ = h.runtime.DeleteSecret(kubeSecretName)
		_ = h.runtime.DeletePersistentVolumeClaim(pvcName)
		_ = h.secretStore.Delete(userID, noryxSecretName)
	}
	if err := h.runtime.CreatePersistentVolumeClaim(noryxruntime.PersistentVolumeClaimSpec{Name: pvcName, StorageClassName: h.workspacePVCClass, Size: req.StorageSize, AccessModes: []string{"ReadWriteOnce"}, Labels: labels}); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to create data service storage: " + err.Error()})
		return
	}
	if err := h.runtime.CreateSecret(noryxruntime.SecretSpec{Name: kubeSecretName, Data: map[string]string{"password": password}, Labels: labels}); err != nil {
		cleanup()
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to create data service credentials: " + err.Error()})
		return
	}
	encrypted, err := security.EncryptString(h.secretsMasterKey, password)
	if err != nil {
		cleanup()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to encrypt data service credentials"})
		return
	}
	if err := h.secretStore.Upsert(secret.New(userID, noryxSecretName, "datasource-managed", encrypted)); err != nil {
		cleanup()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist data service credentials"})
		return
	}
	if err := h.runtime.CreatePod(dataServicePodSpec(resourceName, pvcName, kubeSecretName, req.Database, req.Username, definition, labels, h.registryPullSecret, tier)); err != nil {
		cleanup()
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to create data service pod: " + err.Error()})
		return
	}
	if err := h.runtime.CreateService(noryxruntime.ServiceSpec{Name: resourceName, Selector: map[string]string{"noryx.io/data-service": resourceName}, Port: definition.DefaultPort}); err != nil {
		cleanup()
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to create data service endpoint: " + err.Error()})
		return
	}
	item := datasource.Internal(userID, req.Name, req.Database, req.Username, noryxSecretName, req.StorageSize, tier.ID, definition, resourceName, resourceName, pvcName)
	if err := h.datasourceStore.Create(item); err != nil {
		cleanup()
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to persist internal datasource"})
		return
	}
	h.emitAudit(r, userID, "dataservice.create", "datasource", item.ID, "", "success", "", map[string]any{"name": item.Name, "definitionId": definition.ID, "storageSize": item.StorageSize, "hardwareTier": tier.ID})
	writeJSON(w, http.StatusCreated, item)
}

func (h Handlers) CreateDatasource(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	// La matrice decide (ADR-049).
	if !h.requireCapability(w, auth.Identity{Username: userID, Subject: userID}, platformrole.DeclareDatasource) {
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
	if !supportedDatasourceType(req.Type) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "supported datasource types: postgres, mysql, mongodb"})
		return
	}
	if req.Port <= 0 {
		req.Port = defaultDatasourcePort(req.Type)
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
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	userID := identity.UserID()
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
	// Le proprietaire, pas l'auteur.
	//
	// Cette ligne comparait la personne qui avait declare le connecteur. Une
	// fois le connecteur transferable, les deux se separent : son auteur
	// gardait le droit de le supprimer, et l'equipe a qui on l'avait confie
	// ne l'avait pas.
	if !found || !h.canManageDatasource(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "datasource not found"})
		return
	}
	projectIDs, err := h.projectResourceStore.ListDatasourceProjectIDs(datasourceID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to inspect datasource attachments"})
		return
	}
	if len(projectIDs) > 0 {
		writeJSON(w, http.StatusConflict, map[string]any{"error": "datasource must be detached from every project before deletion", "projectIds": projectIDs})
		return
	}
	if item.Source == "internal" {
		if h.runtime == nil {
			writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "kubernetes runtime is disabled"})
			return
		}
		for _, err := range []error{
			h.runtime.DeleteService(item.ServiceName),
			h.runtime.DeletePod(item.PodName),
			h.runtime.DeleteSecret(item.PodName + "-credentials"),
			h.runtime.DeletePersistentVolumeClaim(item.PVCName),
			h.secretStore.Delete(userID, item.PasswordSecret),
		} {
			if err != nil && !isNotFoundError(err) {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to delete internal data service: " + err.Error()})
				return
			}
		}
	}
	if err := h.datasourceStore.Delete(datasourceID); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to delete datasource"})
		return
	}
	w.WriteHeader(http.StatusNoContent)
	h.emitAudit(r, userID, "datasource.delete", "datasource", item.ID, "", "success", "", map[string]any{"name": item.Name, "source": item.Source})
}

func (h Handlers) enrichDatasourceStatus(item datasource.Datasource) datasource.Datasource {
	if item.Source != "internal" || item.PodName == "" {
		return item
	}
	operator, ok := h.runtime.(noryxruntime.PodOperator)
	if !ok {
		return item
	}
	item.AttachedProjectIDs, _ = h.projectResourceStore.ListDatasourceProjectIDs(item.ID)
	status, err := operator.GetPodStatus(item.PodName)
	if err != nil {
		item.Status = "missing"
		item.StatusReason = "PodMissing"
		item.StatusMessage = err.Error()
		return item
	}
	item.Status = strings.ToLower(status.Phase)
	item.StatusReason = status.Reason
	item.StatusMessage = status.Message
	item.RestartCount = status.RestartCount
	item.StartedAt = status.StartedAt
	return item
}

func (h Handlers) GetDataServiceLogs(w http.ResponseWriter, r *http.Request) {
	item, userID, ok := h.requireOwnedInternalDatasource(w, r)
	if !ok {
		return
	}
	operator, ok := h.runtime.(noryxruntime.PodOperator)
	if !ok {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "data service logs are not supported by runtime"})
		return
	}
	logs, err := operator.GetPodLogs(item.PodName, 500)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to read data service logs: " + err.Error()})
		return
	}
	h.emitAudit(r, userID, "dataservice.logs.read", "datasource", item.ID, "", "success", "", map[string]any{"name": item.Name})
	writeJSON(w, http.StatusOK, map[string]any{"datasourceId": item.ID, "podName": item.PodName, "logs": logs})
}

func (h Handlers) RestartDataService(w http.ResponseWriter, r *http.Request) {
	item, userID, ok := h.requireOwnedInternalDatasource(w, r)
	if !ok {
		return
	}
	operator, ok := h.runtime.(noryxruntime.PodOperator)
	if !ok {
		writeJSON(w, http.StatusNotImplemented, map[string]string{"error": "data service restart is not supported by runtime"})
		return
	}
	if err := operator.RestartPod(item.PodName); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to restart data service: " + err.Error()})
		return
	}
	item.Status = "launching"
	_ = h.datasourceStore.Upsert(item)
	h.emitAudit(r, userID, "dataservice.restart", "datasource", item.ID, "", "success", "", map[string]any{"name": item.Name})
	writeJSON(w, http.StatusAccepted, item)
}

func (h Handlers) requireOwnedInternalDatasource(w http.ResponseWriter, r *http.Request) (datasource.Datasource, string, bool) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return datasource.Datasource{}, "", false
	}
	userID := identity.UserID()
	item, found, err := h.datasourceStore.GetByID(strings.TrimSpace(r.PathValue("datasourceID")))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read datasource"})
		return datasource.Datasource{}, "", false
	}
	if !found || !h.canManageDatasource(item, identity) || item.Source != "internal" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "internal datasource not found"})
		return datasource.Datasource{}, "", false
	}
	return item, userID, true
}

func datasourceDefinitionByID(id string) (datasource.ServiceDefinition, bool) {
	for _, item := range datasource.SystemServiceDefinitions() {
		if item.ID == id {
			return item, true
		}
	}
	return datasource.ServiceDefinition{}, false
}

func generatedDatasourcePassword() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	const alphabet = "abcdefghijkmnopqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"
	for i := range raw {
		raw[i] = alphabet[int(raw[i])%len(alphabet)]
	}
	return string(raw), nil
}

func validStorageSize(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) < 3 || !strings.HasSuffix(value, "Gi") {
		return false
	}
	number := strings.TrimSuffix(value, "Gi")
	if strings.TrimLeft(number, "0") == "" {
		return false
	}
	for _, r := range number {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func dataServicePodSpec(name, pvcName, secretName, database, username string, definition datasource.ServiceDefinition, labels map[string]string, pullSecret string, tier hardware.Tier) noryxruntime.PodSpec {
	env := []noryxruntime.EnvVar{}
	mountPath := "/var/lib/postgresql/data"
	switch definition.Type {
	case "mysql":
		mountPath = "/var/lib/mysql"
		env = append(env,
			noryxruntime.EnvVar{Name: "MYSQL_DATABASE", Value: database},
			noryxruntime.EnvVar{Name: "MYSQL_USER", Value: username},
			noryxruntime.EnvVar{Name: "MYSQL_PASSWORD", SecretName: secretName, SecretKey: "password"},
			noryxruntime.EnvVar{Name: "MYSQL_ROOT_PASSWORD", SecretName: secretName, SecretKey: "password"},
		)
	case "mongodb":
		mountPath = "/data/db"
		env = append(env,
			noryxruntime.EnvVar{Name: "MONGO_INITDB_DATABASE", Value: database},
			noryxruntime.EnvVar{Name: "MONGO_INITDB_ROOT_USERNAME", Value: username},
			noryxruntime.EnvVar{Name: "MONGO_INITDB_ROOT_PASSWORD", SecretName: secretName, SecretKey: "password"},
		)
	default:
		env = append(env,
			noryxruntime.EnvVar{Name: "POSTGRES_DB", Value: database},
			noryxruntime.EnvVar{Name: "POSTGRES_USER", Value: username},
			noryxruntime.EnvVar{Name: "POSTGRES_PASSWORD", SecretName: secretName, SecretKey: "password"},
			noryxruntime.EnvVar{Name: "PGDATA", Value: "/var/lib/postgresql/data/pgdata"},
		)
	}
	return noryxruntime.PodSpec{
		PodName: name, Image: definition.Image, Env: env, Ports: []int{definition.DefaultPort},
		ReadinessPort: definition.DefaultPort, CPURequest: tier.CPURequest, CPULimit: tier.CPULimit, MemRequest: tier.MemoryRequest, MemLimit: tier.MemoryLimit,
		Labels: labels, PullSecret: pullSecret, RestartPolicy: "Always",
		Volumes: []noryxruntime.PersistentVolumeClaimMount{{ClaimName: pvcName, MountPath: mountPath}},
	}
}

func (h Handlers) ValidateDatasource(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	userID := identity.UserID()
	datasourceID := strings.TrimSpace(r.PathValue("datasourceID"))
	item, found, err := h.datasourceStore.GetByID(datasourceID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read datasource"})
		return
	}
	// Tester la connexion releve de l'usage, pas de la propriete.
	if !found || !h.canUseDatasource(item, identity) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "datasource not found"})
		return
	}
	password, err := h.datasourcePassword(item, userID)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read datasource secret: " + err.Error()})
		return
	}
	if item.Type != "postgres" {
		address := net.JoinHostPort(item.Host, fmt.Sprintf("%d", item.Port))
		conn, err := net.DialTimeout("tcp", address, 5*time.Second)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{"reachable": false, "error": err.Error(), "validation": "tcp"})
			return
		}
		_ = conn.Close()
		writeJSON(w, http.StatusOK, map[string]any{"reachable": true, "validation": "tcp"})
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

func supportedDatasourceType(kind string) bool {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "postgres", "mysql", "mongodb":
		return true
	default:
		return false
	}
}

func defaultDatasourcePort(kind string) int {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "mysql":
		return 3306
	case "mongodb":
		return 27017
	default:
		return 5432
	}
}

func (h Handlers) ListProjectDatasources(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	userID := identity.UserID()
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
		// Ce que cette personne peut utiliser ou administrer.
		//
		// La liste rendait l'hote, le port, la base et le compte de chaque
		// connecteur a tout membre du projet, alors que l'injection dans la
		// charge demande, elle, un droit sur le connecteur : on affichait
		// donc une connexion que le workspace ne monterait jamais, avec ses
		// coordonnees.
		if found && h.canSeeDatasource(item, identity) {
			items = append(items, h.enrichDatasourceStatus(item))
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (h Handlers) AttachProjectDatasource(w http.ResponseWriter, r *http.Request) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	userID := identity.UserID()
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	datasourceID := strings.TrimSpace(r.PathValue("datasourceID"))
	if projectID == "" || datasourceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "projectID and datasourceID are required"})
		return
	}
	if !h.requireProjectRole(w, projectID, userID, actionAttachDatasource, "datasource attach") {
		return
	}
	item, found, err := h.datasourceStore.GetByID(datasourceID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read datasource"})
		return
	}
	// Utiliser ou administrer, comme pour un dataset ou une ontologie.
	if !found || !h.canSeeDatasource(item, identity) {
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
	if !h.requireProjectRole(w, projectID, userID, actionAttachDatasource, "datasource detach") {
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
		// Le droit d'usage, pas la paternite.
		//
		// La comparaison portait sur l'auteur : un connecteur transfere a une
		// equipe cessait de fonctionner pour cette equipe tout en continuant
		// de fonctionner pour la personne qui l'avait declare, et un collegue
		// a qui on l'avait donne n'obtenait rien.
		if err != nil || !found || !h.canUseDatasource(item, auth.Identity{Username: userID, Subject: userID}) {
			continue
		}
		password, err := h.datasourcePassword(item, userID)
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

// datasourcePassword resolves the connector's password for whoever is about to
// use it.
//
// A secret belongs to one person: the store is keyed by user and name. So the
// credential of a connector lives with whoever declared it, and lending the
// connector to a team would otherwise hand them a host, a port and no
// password - a grant they could not exercise anywhere, which is a worse
// answer than refusing it. The author's secret is therefore the fallback,
// because lending a connector is lending the credential it carries; that is
// what the grant means, and the permissions screen says so in those words.
//
// The caller's own secret comes first, so somebody who holds the real account
// keeps using it rather than the one the connector was declared with.
func (h Handlers) datasourcePassword(item datasource.Datasource, userID string) (string, error) {
	value, err := h.resolveRepositorySecretValueForWorkspace(userID, item.PasswordSecret)
	if err == nil {
		return value, nil
	}
	author := strings.TrimSpace(item.OwnerUserID)
	if author == "" || strings.EqualFold(author, strings.TrimSpace(userID)) {
		return "", err
	}
	return h.resolveRepositorySecretValueForWorkspace(author, item.PasswordSecret)
}
