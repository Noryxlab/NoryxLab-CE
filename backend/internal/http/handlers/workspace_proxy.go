package handlers

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
)

func (h Handlers) ProxyWorkspace(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserIDFromSessionOrBearer(w, r)
	if !ok {
		return
	}

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

	if !h.requireProjectRole(w, record.ProjectID, userID, access.Role.CanLaunchPod, "workspace access") {
		return
	}

	target, _ := url.Parse("http://" + record.ServiceName + ":8888")
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)

		rest := strings.TrimSpace(r.PathValue("path"))
		targetPath := "/"
		if rest != "" {
			targetPath = "/" + strings.TrimPrefix(rest, "/")
		}
		req.URL.Path = path.Clean(targetPath)
		if targetPath == "/" {
			req.URL.Path = "/"
		}

		q := req.URL.Query()
		if q.Get("token") == "" {
			q.Set("token", record.AccessToken)
		}
		req.URL.RawQuery = q.Encode()
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, err error) {
		writeJSON(rw, http.StatusBadGateway, map[string]string{"error": "workspace proxy failed: " + err.Error()})
	}

	proxy.ServeHTTP(w, r)
}
