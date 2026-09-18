package handlers

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The distinction this file exists to protect: a cell that says nothing
// because the scanner found nothing, and a cell that says nothing because the
// platform was refused, must not be the same value. On EMSE the registry
// account could push and pull images but answered 403 on the scan API, so
// every environment showed an empty column - which reads as reassurance.

func harborHandlers(t *testing.T, handler http.Handler) (Handlers, func()) {
	t.Helper()
	server := httptest.NewServer(handler)
	return Handlers{harborURL: server.URL, harborUsername: "robot", harborPassword: "secret"}, server.Close
}

func imageOn(h Handlers) string {
	return strings.TrimPrefix(strings.TrimPrefix(h.harborURL, "http://"), "https://") + "/noryx-environments/noryx-vscode:1.0.0"
}

func TestVulnerabilitiesReportsARefusalRatherThanNothing(t *testing.T) {
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden} {
		h, closeServer := harborHandlers(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(status)
		}))
		report := h.resolveImageVulnerabilities(imageOn(h))
		closeServer()

		if report == nil {
			t.Fatalf("HTTP %d: the registry refused and the platform reported nothing at all", status)
		}
		if report.Status != scanStatusUnavailable {
			t.Fatalf("HTTP %d: status = %q, want %q", status, report.Status, scanStatusUnavailable)
		}
		if report.Detail == "" {
			t.Fatalf("HTTP %d: no detail, so the screen cannot say why", status)
		}
	}
}

func TestVulnerabilitiesSeparatesUnscannedFromClean(t *testing.T) {
	h, closeServer := harborHandlers(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"digest":"sha256:abc"}`))
	}))
	defer closeServer()

	report := h.resolveImageVulnerabilities(imageOn(h))
	if report == nil || report.Status != scanStatusUnscanned {
		t.Fatalf("an artifact with no scan overview must report %q, got %+v", scanStatusUnscanned, report)
	}
	if report.Total != 0 {
		t.Fatalf("an unscanned image must not carry counts, got total %d", report.Total)
	}
}

func TestVulnerabilitiesReadsTheCountsWhenScanned(t *testing.T) {
	h, closeServer := harborHandlers(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"scan_overview":{"application/vnd.security.vulnerability.report; version=1.1":{"severity":"High","end_time":"2026-09-18T04:00:00Z","summary":{"total":17,"summary":{"Critical":3,"High":14}}}}}`))
	}))
	defer closeServer()

	report := h.resolveImageVulnerabilities(imageOn(h))
	if report == nil || report.Status != scanStatusScanned {
		t.Fatalf("a scanned artifact must report %q, got %+v", scanStatusScanned, report)
	}
	if report.Critical != 3 || report.High != 14 || report.Total != 17 {
		t.Fatalf("counts = %d critical, %d high, %d total; want 3, 14, 17", report.Critical, report.High, report.Total)
	}
	if report.ScannedAt == "" {
		t.Fatal("a scan with an end time must say when it ran")
	}
}

// An image the registry does not hold is the one case where saying nothing is
// right: the platform has no standing to report on it.
func TestVulnerabilitiesStaysSilentAboutForeignImages(t *testing.T) {
	h, closeServer := harborHandlers(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("the registry was queried about an image it does not hold")
	}))
	defer closeServer()

	if report := h.resolveImageVulnerabilities("docker.io/library/python:3.12"); report != nil {
		t.Fatalf("a foreign image must yield no report, got %+v", report)
	}
}
