package handlers

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
)

const workspaceTokenCookiePrefix = "noryx_ws_token_"

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

	userID, hasUser := h.userIDFromSessionOrBearerNoWrite(r)
	queryToken := strings.TrimSpace(r.URL.Query().Get("token"))
	wsCookieName := workspaceTokenCookiePrefix + workspaceID
	wsCookieToken := ""
	if cookie, err := r.Cookie(wsCookieName); err == nil {
		wsCookieToken = strings.TrimSpace(cookie.Value)
	}

	tokenAuth := queryToken != "" && queryToken == record.AccessToken
	cookieAuth := wsCookieToken != "" && wsCookieToken == record.AccessToken
	if hasUser {
		if !h.requireProjectRole(w, record.ProjectID, userID, access.Role.CanLaunchPod, "workspace access") {
			return
		}
	} else if !(tokenAuth || cookieAuth) {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authenticated session"})
		return
	}

	if tokenAuth && !cookieAuth {
		http.SetCookie(w, &http.Cookie{
			Name:     wsCookieName,
			Value:    record.AccessToken,
			Path:     "/workspaces/" + workspaceID + "/",
			HttpOnly: true,
			Secure:   true,
			SameSite: http.SameSiteLaxMode,
			Expires:  time.Now().UTC().Add(8 * time.Hour),
		})
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

		rest := strings.TrimSpace(r.PathValue("path"))
		targetPath := "/workspaces/" + workspaceID
		if rest != "" {
			targetPath = "/workspaces/" + workspaceID + "/" + strings.TrimPrefix(rest, "/")
		}
		req.URL.Path = targetPath
		req.Header.Set("X-Forwarded-Proto", "https")
		req.Header.Set("X-Forwarded-Host", r.Host)
		req.Header.Set("X-Forwarded-Port", "443")
		req.Header.Set("X-Forwarded-Prefix", "/workspaces/"+workspaceID)
		req.Header.Set("X-Forwarded-For", r.RemoteAddr)
		// Keep public host so Jupyter builds browser-facing URLs under datalab.noryxlab.ai.
		req.Host = r.Host

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
