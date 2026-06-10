package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

func (h Handlers) ProxyProjectFiles(w http.ResponseWriter, r *http.Request) {
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return
	}
	if !h.requireProjectMember(w, projectID, identity.UserID(), "project file access") {
		return
	}
	if r.Method != http.MethodGet && !h.requireProjectRole(w, projectID, identity.UserID(), access.Role.CanLaunchPod, "project file modification") {
		return
	}
	auditPath := strings.Trim(strings.TrimSpace(r.PathValue("path")), "/")
	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read folder request"})
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		var request struct {
			Path string `json:"path"`
		}
		if json.Unmarshal(body, &request) == nil {
			auditPath = strings.Trim(strings.TrimSpace(request.Path), "/")
		}
	}
	serviceName, err := h.ensureProjectFileService(projectID)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "project file service unavailable: " + err.Error()})
		return
	}

	target, _ := url.Parse("http://" + serviceName + "." + h.workspaceNamespace + ".svc.cluster.local:8080")
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		if r.Method == http.MethodPost {
			req.URL.Path = "/folders"
		} else {
			req.URL.Path = "/files"
			if rest := strings.Trim(strings.TrimSpace(r.PathValue("path")), "/"); rest != "" {
				req.URL.Path += "/" + rest
			}
		}
		req.Host = target.Host
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, err error) {
		if r.Method != http.MethodGet {
			h.emitAdvancedAudit(r, identity.UserID(), "project.file."+strings.ToLower(r.Method), "project", projectID, projectID, "failure", "proxy_failed", projectFileAuditDetails(auditPath))
		}
		writeJSON(rw, http.StatusBadGateway, map[string]string{"error": "project file proxy failed: " + err.Error()})
	}
	if r.Method != http.MethodGet {
		proxy.ModifyResponse = func(response *http.Response) error {
			outcome := "success"
			errorCode := ""
			if response.StatusCode < 200 || response.StatusCode >= 300 {
				outcome = "failure"
				errorCode = "file_operation_failed"
			}
			h.emitAdvancedAudit(r, identity.UserID(), "project.file."+strings.ToLower(r.Method), "project", projectID, projectID, outcome, errorCode, projectFileAuditDetails(auditPath))
			return nil
		}
	}
	proxy.ServeHTTP(w, r)
}

func projectFileAuditDetails(path string) map[string]any {
	return map[string]any{"path": strings.Trim(strings.TrimSpace(path), "/")}
}

func (h Handlers) ensureProjectFileService(projectID string) (string, error) {
	if h.runtime == nil {
		return "", fmt.Errorf("kubernetes runtime is disabled")
	}
	projectID = strings.TrimSpace(projectID)
	name := "project-files-" + sanitizeK8sName(projectID)
	pvcName := "project-" + sanitizeK8sName(projectID)
	if err := h.runtime.CreatePersistentVolumeClaim(noryxruntime.PersistentVolumeClaimSpec{
		Name:             pvcName,
		StorageClassName: h.workspacePVCClass,
		Size:             h.workspacePVCSize,
		AccessModes:      []string{h.workspacePVCAccessMode},
		Labels:           map[string]string{"app.kubernetes.io/name": "noryx-project-files", "noryx.io/project-id": projectID},
	}); err != nil {
		return "", err
	}
	readiness, hasReadiness := h.runtime.(noryxruntime.WorkspaceReadiness)
	if hasReadiness {
		if ready, _ := readiness.IsServiceReady(name); ready {
			return name, nil
		}
	}
	// Idle project file pods exit by design. Remove the completed/stale pod so
	// the same stable service name can be recreated on demand.
	if err := h.runtime.DeletePod(name); err != nil && !isNotFoundError(err) {
		return "", err
	}
	labels := map[string]string{
		"app.kubernetes.io/name":  "noryx-project-files",
		"noryx.io/project-id":     projectID,
		"noryx.io/project-files":  name,
		"noryx.io/workload-class": "technical",
	}
	if err := h.runtime.CreatePod(noryxruntime.PodSpec{
		PodName:       name,
		Image:         h.projectFilesImage,
		Command:       []string{"/usr/local/bin/noryx-project-files"},
		Args:          []string{"--root=/mnt", "--listen=:8080", "--idle-timeout=15m"},
		Ports:         []int{8080},
		ReadinessPort: 8080,
		CPURequest:    "25m",
		CPULimit:      "250m",
		MemRequest:    "32Mi",
		MemLimit:      "128Mi",
		PullSecret:    h.registryPullSecret,
		RunAsUser:     1000,
		RunAsGroup:    1000,
		FSGroup:       1000,
		Volumes: []noryxruntime.PersistentVolumeClaimMount{{
			ClaimName: pvcName,
			MountPath: "/mnt",
		}},
		Labels: labels,
	}); err != nil && !strings.Contains(err.Error(), "status=409") {
		return "", err
	}
	if err := h.runtime.CreateService(noryxruntime.ServiceSpec{Name: name, Selector: map[string]string{"noryx.io/project-files": name}, Port: 8080}); err != nil && !strings.Contains(err.Error(), "status=409") {
		return "", err
	}
	if !hasReadiness {
		return name, nil
	}
	for range 160 {
		if ready, _ := readiness.IsServiceReady(name); ready {
			return name, nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return "", fmt.Errorf("service %s did not become ready", name)
}
