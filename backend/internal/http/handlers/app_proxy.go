package handlers

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
)

func (h Handlers) ProxyApp(w http.ResponseWriter, r *http.Request) {
	slug := normalizeAppSlug(r.PathValue("slug"))
	if slug == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "slug is required"})
		return
	}
	record, found, err := h.appStore.GetBySlug(slug)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read app"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "app not found"})
		return
	}
	userID, ok := h.requireUserIDFromSessionOrBearer(w, r)
	if !ok {
		return
	}
	if !h.requireProjectRole(w, record.ProjectID, userID, access.Role.CanLaunchPod, "app access") {
		return
	}

	targetHost := strings.TrimSpace(record.ServiceName)
	if targetHost == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "app service is not configured"})
		return
	}
	if !strings.Contains(targetHost, ".") {
		namespace := strings.TrimSpace(h.workspaceNamespace)
		if namespace == "" {
			namespace = "default"
		}
		targetHost = targetHost + "." + namespace + ".svc.cluster.local"
	}
	target, _ := url.Parse("http://" + targetHost + ":" + strconv.Itoa(record.Port))
	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		rest := strings.TrimSpace(r.PathValue("path"))
		targetPath := "/"
		if rest != "" {
			targetPath = "/" + strings.TrimPrefix(rest, "/")
		}
		req.URL.Path = targetPath
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", r.Host)
		req.Header.Set("X-Forwarded-Port", "443")
		req.Header.Set("X-Forwarded-Prefix", "/apps/"+record.Slug)
		req.Header.Set("X-Forwarded-For", r.RemoteAddr)
		req.Host = r.Host
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, err error) {
		writeJSON(rw, http.StatusBadGateway, map[string]string{"error": "app proxy failed: " + err.Error()})
	}
	proxy.ServeHTTP(w, r)
}
