package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func validateRepositoryConnectivity(repoURL, secretValue string) error {
	host, repoPath, err := parseRepoLocation(repoURL)
	if err != nil {
		return err
	}

	client := &http.Client{Timeout: 12 * time.Second}
	reqURL := ""
	switch host {
	case "github.com":
		reqURL = "https://api.github.com/repos/" + repoPath
	case "gitlab.com":
		reqURL = "https://gitlab.com/api/v4/projects/" + url.PathEscape(repoPath)
	default:
		// Generic smart-HTTP git endpoint probe.
		base := strings.TrimSuffix(strings.TrimSpace(repoURL), ".git")
		reqURL = strings.TrimSuffix(base, "/") + ".git/info/refs?service=git-upload-pack"
	}

	req, err := http.NewRequest(http.MethodGet, reqURL, nil)
	if err != nil {
		return fmt.Errorf("invalid validation request")
	}
	req.Header.Set("User-Agent", "noryx-ce-repo-validator")
	applyRepositoryAuthHeaders(req, host, secretValue)

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("unreachable")
	}
	_ = resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("authentication failed (status=%d)", resp.StatusCode)
	case http.StatusNotFound:
		return fmt.Errorf("repository not found")
	default:
		return fmt.Errorf("unexpected status=%d", resp.StatusCode)
	}
}

func applyRepositoryAuthHeaders(req *http.Request, host, secretValue string) {
	secretValue = strings.TrimSpace(secretValue)
	if secretValue == "" {
		return
	}

	if creds := strings.SplitN(secretValue, ":", 2); len(creds) == 2 && creds[0] != "" {
		req.SetBasicAuth(creds[0], creds[1])
	} else {
		req.Header.Set("Authorization", "Bearer "+secretValue)
	}
	if host == "gitlab.com" {
		req.Header.Set("PRIVATE-TOKEN", secretValue)
	}
}

