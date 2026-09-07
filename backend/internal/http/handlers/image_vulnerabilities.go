package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// What the registry found in an image.
//
// Rebuilding an environment weekly picks up the base image's security fixes.
// That is a ritual until somebody can see whether it worked: "we rebuild" is
// an intention, "3 critical, 14 high" is a fact. Harbor scans on push with
// Trivy and reports the counts on the artifact the platform already asks about
// to pin its digest, so this costs one more field on a call already being
// made.
//
// Nothing here blocks anything. An image with vulnerabilities still runs -
// refusing to start a workspace because a scanner found a high-severity CVE in
// a library nothing calls is how a platform teaches people to work around it.

type imageVulnerabilities struct {
	Critical  int    `json:"critical"`
	High      int    `json:"high"`
	Medium    int    `json:"medium"`
	Low       int    `json:"low"`
	Unknown   int    `json:"unknown"`
	Total     int    `json:"total"`
	Severity  string `json:"severity,omitempty"`
	ScannedAt string `json:"scannedAt,omitempty"`
}

// resolveImageVulnerabilities answers with what the registry knows, or nothing
// at all. Nothing is the honest answer when scanning is not enabled: an empty
// report must never read as a clean one.
func (h Handlers) resolveImageVulnerabilities(image string) *imageVulnerabilities {
	image = strings.TrimSpace(image)
	if image == "" {
		return nil
	}
	registry, repositoryPath, reference, ok := splitImageReference(image)
	if !ok || !strings.EqualFold(registry, registryHost(h.harborURL)) {
		return nil
	}
	project, repository, ok := splitHarborRepository(repositoryPath)
	if !ok {
		return nil
	}

	endpoint := fmt.Sprintf("%s/api/v2.0/projects/%s/repositories/%s/artifacts/%s?with_scan_overview=true",
		strings.TrimSuffix(h.harborURL, "/"),
		url.PathEscape(project),
		url.PathEscape(repository),
		url.PathEscape(reference),
	)
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil
	}
	if user, password := strings.TrimSpace(h.harborUsername), strings.TrimSpace(h.harborPassword); user != "" && password != "" {
		request.SetBasicAuth(user, password)
	}

	response, err := h.harborHTTPClient(5 * time.Second).Do(request)
	if err != nil {
		log.Printf("vulnerabilities: cannot reach the registry for %s: %v", image, err)
		return nil
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil
	}

	// Harbor keys the overview by report mime type, which varies with the
	// scanner and its version, so the map is read rather than a fixed key.
	var artifact struct {
		ScanOverview map[string]struct {
			Severity string    `json:"severity"`
			EndTime  time.Time `json:"end_time"`
			Summary  struct {
				Total   int            `json:"total"`
				Summary map[string]int `json:"summary"`
			} `json:"summary"`
		} `json:"scan_overview"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&artifact); err != nil {
		return nil
	}
	for _, overview := range artifact.ScanOverview {
		report := &imageVulnerabilities{
			Critical: overview.Summary.Summary["Critical"],
			High:     overview.Summary.Summary["High"],
			Medium:   overview.Summary.Summary["Medium"],
			Low:      overview.Summary.Summary["Low"],
			Unknown:  overview.Summary.Summary["Unknown"],
			Total:    overview.Summary.Total,
			Severity: overview.Severity,
		}
		if !overview.EndTime.IsZero() {
			report.ScannedAt = overview.EndTime.UTC().Format(time.RFC3339)
		}
		return report
	}
	return nil
}
