package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

func (h Handlers) GetBuildDockerfile(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}

	h.syncBuildsFromRuntime()

	buildID := strings.TrimSpace(r.PathValue("buildID"))
	if buildID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "buildID is required"})
		return
	}

	record, found, err := h.buildStore.GetByID(buildID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read build"})
		return
	}
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "build not found"})
		return
	}
	if _, allowed := h.accessStore.GetRole(record.ProjectID, userID); !allowed {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient role for environment access"})
		return
	}

	content, sourceURL, err := fetchDockerfileContent(record.GitRepository, record.GitRef, record.DockerfilePath)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "dockerfile fetch failed: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"buildId":        record.ID,
		"projectId":      record.ProjectID,
		"gitRepository":  record.GitRepository,
		"gitRef":         record.GitRef,
		"dockerfilePath": record.DockerfilePath,
		"sourceUrl":      sourceURL,
		"content":        content,
	})
}

func fetchDockerfileContent(repo, gitRef, dockerfilePath string) (string, string, error) {
	host, repoPath, err := parseRepoLocation(repo)
	if err != nil {
		return "", "", err
	}
	filePath, err := cleanRelativePath(dockerfilePath)
	if err != nil {
		return "", "", err
	}

	refs := candidateRefs(gitRef)
	client := &http.Client{Timeout: 15 * time.Second}
	lastErr := error(nil)
	for _, ref := range refs {
		rawURL, err := buildRawDockerfileURL(host, repoPath, ref, filePath)
		if err != nil {
			return "", "", err
		}
		req, err := http.NewRequest(http.MethodGet, rawURL, nil)
		if err != nil {
			return "", "", err
		}
		req.Header.Set("User-Agent", "noryx-ce-dockerfile-fetcher")
		req.Header.Set("Accept", "text/plain")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("status=%d", resp.StatusCode)
			continue
		}
		return string(body), rawURL, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("unable to fetch dockerfile")
	}
	return "", "", lastErr
}

func parseRepoLocation(repo string) (string, string, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return "", "", fmt.Errorf("gitRepository is empty")
	}

	var host, repoPath string
	switch {
	case strings.HasPrefix(repo, "git@"):
		// git@github.com:org/repo.git
		parts := strings.SplitN(strings.TrimPrefix(repo, "git@"), ":", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("unsupported repository format")
		}
		host = strings.ToLower(strings.TrimSpace(parts[0]))
		repoPath = strings.TrimSpace(parts[1])
	case strings.Contains(repo, "://"):
		u, err := url.Parse(repo)
		if err != nil {
			return "", "", fmt.Errorf("invalid repository url")
		}
		host = strings.ToLower(strings.TrimSpace(u.Host))
		repoPath = strings.TrimPrefix(strings.TrimSpace(u.Path), "/")
	default:
		return "", "", fmt.Errorf("unsupported repository format")
	}

	repoPath = strings.TrimSuffix(repoPath, ".git")
	repoPath = strings.Trim(repoPath, "/")
	if host == "" || repoPath == "" {
		return "", "", fmt.Errorf("invalid repository location")
	}
	return host, repoPath, nil
}

func cleanRelativePath(input string) (string, error) {
	cleaned := path.Clean("/" + strings.TrimSpace(input))
	if cleaned == "/" || strings.HasPrefix(cleaned, "/..") {
		return "", fmt.Errorf("invalid dockerfilePath")
	}
	return strings.TrimPrefix(cleaned, "/"), nil
}

func candidateRefs(ref string) []string {
	ref = strings.TrimSpace(ref)
	ref = strings.TrimPrefix(ref, "refs/heads/")
	ref = strings.TrimPrefix(ref, "refs/tags/")
	if ref == "" {
		return []string{"main", "master"}
	}
	if ref == "main" {
		return []string{"main", "master"}
	}
	return []string{ref}
}

func buildRawDockerfileURL(host, repoPath, gitRef, dockerfilePath string) (string, error) {
	switch host {
	case "github.com":
		parts := strings.Split(repoPath, "/")
		if len(parts) < 2 {
			return "", fmt.Errorf("invalid github repository path")
		}
		repoKey := parts[0] + "/" + parts[1]
		return "https://raw.githubusercontent.com/" + repoKey + "/" + gitRef + "/" + dockerfilePath, nil
	case "gitlab.com":
		return "https://gitlab.com/" + repoPath + "/-/raw/" + gitRef + "/" + dockerfilePath, nil
	default:
		return "", fmt.Errorf("unsupported git host: %s", host)
	}
}
