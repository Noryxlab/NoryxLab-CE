package handlers

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
)

func (h Handlers) ProxyWorkspace(w http.ResponseWriter, r *http.Request) {
	workspaceID := strings.TrimSpace(r.PathValue("workspaceID"))
	if workspaceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "workspaceID is required"})
		return
	}

	record, found, err := h.workspaceStore.GetByID(workspaceID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read workspace"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "workspace not found"})
		return
	}

	identity, ok := h.requireIdentityFromSessionOrBearer(w, r)
	if !ok {
		return
	}
	if !h.requireProjectRole(w, record.ProjectID, identity.UserID(), access.Role.CanLaunchPod, "workspace access") {
		return
	}

	targetHost := strings.TrimSpace(record.ServiceName)
	if targetHost == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "workspace service is not configured"})
		return
	}
	if !strings.Contains(targetHost, ".") {
		namespace := strings.TrimSpace(h.workspaceNamespace)
		if namespace == "" {
			namespace = "default"
		}
		targetHost = targetHost + "." + namespace + ".svc.cluster.local"
	}

	target, _ := url.Parse("http://" + targetHost + ":8888")
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		req.URL.Path = workspaceProxyTargetPath(record.Kind, workspaceID, r.PathValue("path"))
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", r.Host)
		req.Header.Set("X-Forwarded-Port", "443")
		req.Header.Set("X-Forwarded-Prefix", "/workspaces/"+workspaceID)
		req.Header.Set("X-Forwarded-For", r.RemoteAddr)
		// Keep public host so Jupyter builds browser-facing URLs under datalab.noryxlab.ai.
		req.Host = r.Host

		if normalizeWorkspaceKind(record.Kind) == "jupyter" {
			q := req.URL.Query()
			// The Jupyter token is internal to the Noryx-to-workspace hop.
			q.Set("token", record.AccessToken)
			req.URL.RawQuery = q.Encode()
		}
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, err error) {
		writeJSON(rw, http.StatusBadGateway, map[string]string{"error": "workspace proxy failed: " + err.Error()})
	}

	proxy.ServeHTTP(w, r)
}

func workspaceProxyTargetPath(kind, workspaceID, rest string) string {
	rest = strings.TrimSpace(rest)
	// RStudio uses www-root-path to generate browser-facing URLs, but expects
	// the reverse proxy to strip that public prefix before forwarding.
	if normalizeWorkspaceKind(kind) == "rstudio" {
		if rest == "" {
			return "/"
		}
		return "/" + strings.TrimPrefix(rest, "/")
	}

	targetPath := "/workspaces/" + workspaceID
	if rest != "" {
		targetPath += "/" + strings.TrimPrefix(rest, "/")
	}
	return targetPath
}
