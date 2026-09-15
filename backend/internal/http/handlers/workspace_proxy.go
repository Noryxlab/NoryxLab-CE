package handlers

import (
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/workspacekind"
)

func (h Handlers) ProxyWorkspace(w http.ResponseWriter, r *http.Request) {
	workspaceID := strings.TrimSpace(r.PathValue("workspaceID"))
	if workspaceID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "workspaceID is required"})
		return
	}

	// Who is asking comes first.
	//
	// The lookup used to run before this, so the endpoint answered 404 for an
	// identifier that does not exist and 401 for one that does - telling a
	// stranger which workspaces are real without ever signing in. The
	// identifiers are hard to guess, which is a reason this was never
	// exploited and not a reason to leave it.
	identity, ok := h.requireIdentityFromSessionOrBearer(w, r)
	if !ok {
		return
	}

	record, found, err := h.workspaceStore.GetByID(workspaceID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read workspace"})
		return
	}
	if !found {
		writeWorkspaceGone(w, r)
		return
	}
	if !h.requireProjectRole(w, record.ProjectID, identity.UserID(), actionLaunch, "workspace access") {
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
		// Keep public host so Jupyter builds browser-facing URLs under datalab.example.local.
		req.Host = r.Host

		if normalizeWorkspaceKind(record.Kind) == "jupyter" {
			q := req.URL.Query()
			// The Jupyter token is internal to the Noryx-to-workspace hop.
			q.Set("token", record.AccessToken)
			req.URL.RawQuery = q.Encode()
		}
	}
	proxy.ErrorHandler = func(rw http.ResponseWriter, _ *http.Request, err error) {
		// The workspace did not answer. Before reporting a transport error,
		// ask the pod why - because the commonest reason is one the platform
		// already knows how to explain and the user cannot deduce.
		//
		// A workspace killed for memory disappears. To somebody inside it,
		// clicking, that is a connection refused: no notice, no cause, no
		// remedy. The out-of-memory message existed before this and appeared
		// only in the workspace list, which is the one place a person in this
		// situation is not looking.
		if notice, ok := h.workspaceFailureNotice(record.PodName); ok {
			writeWorkspaceStopped(rw, r, notice)
			return
		}
		writeWorkspaceStopped(rw, r, "")
		_ = err
	}

	proxy.ServeHTTP(w, r)
}

func workspaceProxyTargetPath(kind, workspaceID, rest string) string {
	rest = strings.TrimSpace(rest)
	// RStudio uses www-root-path to generate browser-facing URLs, but expects
	// the reverse proxy to strip that public prefix before forwarding.
	if registered, ok := workspacekind.Lookup(kind); ok {
		if !registered.StripProxyPrefix {
			targetPath := "/workspaces/" + workspaceID
			if rest != "" {
				targetPath += "/" + strings.TrimPrefix(rest, "/")
			}
			return targetPath
		}
		if rest == "" {
			return "/"
		}
		return "/" + strings.TrimPrefix(rest, "/")
	}
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

// writeWorkspaceGone answers a workspace that is no longer there.
//
// This address is one a person keeps: it is what a browser bookmarks and what
// gets pasted into a message. A workspace is deleted far more often than it is
// renamed, so the common way to arrive here is an old link - and the answer
// was a bare {"error":"workspace not found"} rendered as text in the address
// bar, which reads as a broken platform rather than a stale link.
//
// A browser asking for a document gets a page saying what happened and where
// to go. Anything else - a script, a fetch from the application - keeps the
// JSON it expects.
func writeWorkspaceGone(w http.ResponseWriter, r *http.Request) {
	if !strings.Contains(r.Header.Get("Accept"), "text/html") {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "workspace not found"})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte(workspaceGonePage))
}

// Deliberately one self-contained page with no asset of its own: it is served
// when something is already missing, and a stylesheet that also fails to load
// would make the explanation look like a second fault.
const workspaceGonePage = `<!doctype html>
<html lang="fr">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Espace de travail introuvable</title>
<style>
  :root { color-scheme: light dark; }
  body { margin: 0; min-height: 100dvh; display: grid; place-items: center;
    background: #f7f8fa; color: #10192b; padding: 1.5rem;
    font: 0.875rem/1.6 'Plus Jakarta Sans', ui-sans-serif, system-ui, -apple-system,
      'Segoe UI', Roboto, sans-serif; }
  @media (prefers-color-scheme: dark) { body { background: #0b1017; color: #e8edf5; } }
  main { max-width: 34rem; text-align: center; }
  h1 { font-size: 1.125rem; font-weight: 600; margin: 0 0 0.5rem; }
  p { margin: 0 0 1rem; color: #56637c; }
  @media (prefers-color-scheme: dark) { p { color: #97a3b8; } }
  a { display: inline-block; padding: 0.4375rem 0.875rem; border-radius: 0.375rem;
    background: #0b5fd0; color: #fff; text-decoration: none; font-weight: 500; }
  @media (prefers-color-scheme: dark) { a { background: #1684ff; color: #05192f; } }
</style>
</head>
<body>
<main>
  <h1>Cet espace de travail n&rsquo;existe plus</h1>
  <p>Il a &eacute;t&eacute; supprim&eacute;, ou ce lien date d&rsquo;avant sa suppression.
     Les fichiers enregistr&eacute;s dans le projet ou dans votre dossier personnel
     sont conserv&eacute;s ; seul l&rsquo;espace de travail lui-m&ecirc;me est parti.</p>
  <a href="/">Retour &agrave; la plateforme</a>
</main>
</body>
</html>
`

// workspaceFailureNotice asks the runtime why a workspace stopped answering.
//
// Only an answer the platform is sure of is returned. A pod that is simply
// starting, or one the runtime cannot describe, gets the generic page: a wrong
// cause stated confidently is worse than no cause, because it sends somebody
// to change the one thing that was not the problem.
func (h Handlers) workspaceFailureNotice(podName string) (string, bool) {
	if strings.TrimSpace(podName) == "" {
		return "", false
	}
	operator, ok := h.runtime.(noryxruntime.PodOperator)
	if !ok {
		return "", false
	}
	status, err := operator.GetPodStatus(podName)
	if err != nil {
		return "", false
	}
	return outOfMemoryStatus(status)
}

// writeWorkspaceStopped answers a workspace that is there but not serving.
//
// Separate from the deleted case because the remedy differs: a deleted
// workspace is gone and the link is stale, while this one can be started
// again - and if it was killed for memory, started again at a larger size,
// which is the only action that changes the outcome.
func writeWorkspaceStopped(w http.ResponseWriter, r *http.Request, notice string) {
	message := notice
	if message == "" {
		message = "L'espace de travail ne répond pas. Il est peut-être en train de démarrer, ou il s'est arrêté."
	}
	if !strings.Contains(r.Header.Get("Accept"), "text/html") {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": message})
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusBadGateway)
	_, _ = w.Write([]byte(strings.Replace(workspaceStoppedPage, "{{message}}", htmlEscape(message), 1)))
}

func htmlEscape(value string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return replacer.Replace(value)
}

const workspaceStoppedPage = `<!doctype html>
<html lang="fr">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Espace de travail arrêté</title>
<style>
  :root { color-scheme: light dark; }
  body { margin: 0; min-height: 100dvh; display: grid; place-items: center;
    background: #f7f8fa; color: #10192b; padding: 1.5rem;
    font: 0.875rem/1.6 'Plus Jakarta Sans', ui-sans-serif, system-ui, -apple-system,
      'Segoe UI', Roboto, sans-serif; }
  @media (prefers-color-scheme: dark) { body { background: #0b1017; color: #e8edf5; } }
  main { max-width: 34rem; text-align: center; }
  h1 { font-size: 1.125rem; font-weight: 600; margin: 0 0 0.5rem; }
  p { margin: 0 0 1rem; color: #56637c; }
  @media (prefers-color-scheme: dark) { p { color: #97a3b8; } }
  a { display: inline-block; padding: 0.4375rem 0.875rem; border-radius: 0.375rem;
    background: #0b5fd0; color: #fff; text-decoration: none; font-weight: 500; }
  @media (prefers-color-scheme: dark) { a { background: #1684ff; color: #05192f; } }
</style>
</head>
<body>
<main>
  <h1>L&rsquo;espace de travail ne r&eacute;pond plus</h1>
  <p>{{message}}</p>
  <a href="/">Retour &agrave; la plateforme</a>
</main>
</body>
</html>
`
