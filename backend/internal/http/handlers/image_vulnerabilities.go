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

// Status separates three answers a screen must never merge: the registry
// scanned this image, the registry has not scanned it, and the platform could
// not ask. Only the first makes the counts meaningful, and the third is not an
// absence of findings - it is an absence of knowledge, which on a platform that
// holds regulated data is the one that has to be visible.
const (
	scanStatusScanned     = "scanned"
	scanStatusUnscanned   = "unscanned"
	scanStatusUnavailable = "unavailable"
)

type imageVulnerabilities struct {
	Status    string `json:"status"`
	Detail    string `json:"detail,omitempty"`
	Critical  int    `json:"critical"`
	High      int    `json:"high"`
	Medium    int    `json:"medium"`
	Low       int    `json:"low"`
	Unknown   int    `json:"unknown"`
	Total     int    `json:"total"`
	Severity  string `json:"severity,omitempty"`
	ScannedAt string `json:"scannedAt,omitempty"`
}

// resolveImageVulnerabilities answers with what the registry knows, why it
// knows nothing, or nothing at all.
//
// Nothing at all is reserved for an image this registry does not hold: the
// platform has no standing to say anything about it. For its own images the
// answer is always a report, because the alternative is a column that renders
// an em dash whether the scanner found nothing or the credential was refused -
// and those read identically while meaning opposite things. EMSE ran that way:
// the robot account could push and pull but got 403 on the scan API, so every
// environment showed an empty cell and nobody had reason to look.
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
	unavailable := func(detail string) *imageVulnerabilities {
		return &imageVulnerabilities{Status: scanStatusUnavailable, Detail: detail}
	}

	endpoint := fmt.Sprintf("%s/api/v2.0/projects/%s/repositories/%s/artifacts/%s?with_scan_overview=true",
		strings.TrimSuffix(h.harborURL, "/"),
		url.PathEscape(project),
		url.PathEscape(repository),
		url.PathEscape(reference),
	)
	request, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return unavailable("the registry request could not be built")
	}
	if user, password := strings.TrimSpace(h.harborUsername), strings.TrimSpace(h.harborPassword); user != "" && password != "" {
		request.SetBasicAuth(user, password)
	}

	response, err := h.harborHTTPClient(5 * time.Second).Do(request)
	if err != nil {
		log.Printf("vulnerabilities: cannot reach the registry for %s: %v", image, err)
		return unavailable("the registry could not be reached")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		// Logged every time rather than once: a permission the registry
		// administrator has to grant is not going to be noticed in a cell that
		// says "unavailable", and this is how the 403 on EMSE was found.
		log.Printf("vulnerabilities: the registry refused the scan report for %s: HTTP %d", image, response.StatusCode)
		switch {
		case response.StatusCode == http.StatusUnauthorized, response.StatusCode == http.StatusForbidden:
			return unavailable("the platform's registry account may not read scan reports")
		case response.StatusCode == http.StatusNotFound:
			return unavailable("the registry does not hold this image")
		default:
			return unavailable(fmt.Sprintf("the registry answered HTTP %d", response.StatusCode))
		}
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
		return unavailable("the registry's answer could not be read")
	}
	for _, overview := range artifact.ScanOverview {
		report := &imageVulnerabilities{
			Status:   scanStatusScanned,
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
	// Reached the artifact, and it carries no scan overview: the image exists
	// and has not been scanned. Distinct from every branch above.
	return &imageVulnerabilities{Status: scanStatusUnscanned}
}
