package handlers

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/app"
)

func (h Handlers) ProxyApp(w http.ResponseWriter, r *http.Request) {
	isDashboardRoute := strings.HasPrefix(r.URL.Path, "/dashboards/")
	expectedKind := "app"
	forwardedPrefix := "/apps/"
	if isDashboardRoute {
		expectedKind = "dashboard"
		forwardedPrefix = "/dashboards/"
	}

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
	if strings.TrimSpace(record.Kind) == "" {
		record.Kind = "app"
	}
	if record.Kind != expectedKind {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "resource not found"})
		return
	}
	if !h.requireAppAccess(w, r, record) {
		return
	}
	if r.Method == http.MethodGet && strings.TrimSpace(r.PathValue("path")) == "" {
		actorUserID, ok := h.userIDFromSessionOrBearerNoWrite(r)
		if !ok {
			actorUserID = "anonymous"
		}
		h.emitAudit(r, actorUserID, "app.view", record.Kind, record.ID, record.ProjectID, "success", "", map[string]any{
			"slug":       record.Slug,
			"accessMode": record.AccessMode,
		})
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
		req.Header.Set("X-Forwarded-Prefix", forwardedPrefix+record.Slug)
		req.Header.Set("X-Forwarded-For", r.RemoteAddr)
		req.Host = r.Host
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, err error) {
		writeJSON(rw, http.StatusBadGateway, map[string]string{"error": "app proxy failed: " + err.Error()})
	}
	proxy.ServeHTTP(w, r)
}

func (h Handlers) requireAppAccess(w http.ResponseWriter, r *http.Request, record app.App) bool {
	mode := strings.ToLower(strings.TrimSpace(record.AccessMode))
	if mode == "public" {
		return true
	}
	identity, ok := h.requireIdentityFromSessionOrBearer(w, r)
	if !ok {
		return false
	}
	if h.isGlobalAdmin(identity) {
		return true
	}
	switch mode {
	case "private":
		if appIdentityMatches(identity, record.OwnerUserID) {
			return true
		}
	case "users":
		for _, userID := range record.AllowedUsers {
			if appIdentityMatches(identity, userID) {
				return true
			}
		}
	case "organization":
		for _, organizationID := range record.AllowedOrganizations {
			if h.userBelongsToOrganization(identity.Subject, organizationID) || h.userBelongsToOrganization(identity.UserID(), organizationID) {
				return true
			}
		}
	default:
		return h.requireProjectRole(w, record.ProjectID, identity.UserID(), access.Role.CanLaunchPod, "app access")
	}
	writeJSON(w, http.StatusForbidden, map[string]string{"error": "app access denied"})
	return false
}

func appIdentityMatches(identity auth.Identity, expected string) bool {
	expected = strings.TrimSpace(expected)
	return expected != "" && (strings.EqualFold(identity.UserID(), expected) || strings.EqualFold(identity.Subject, expected) || identity.MatchesUsername(expected) || identity.MatchesEmail(expected))
}
