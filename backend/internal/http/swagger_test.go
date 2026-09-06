package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The reader of an installation with no outbound route is the one who needs
// this page most: an air-gapped customer cannot read the API any other way.
func TestSwaggerUINeedsNothingFromTheInternet(t *testing.T) {
	recorder := httptest.NewRecorder()
	GetSwaggerUI(recorder, httptest.NewRequest(http.MethodGet, "/swagger", nil))
	body := recorder.Body.String()
	for _, host := range []string{"unpkg.com", "cdn.jsdelivr.net", "cdnjs.cloudflare.com", "//fonts."} {
		if strings.Contains(body, host) {
			t.Errorf("the Swagger page loads %s; an installation behind a VPN would show a blank screen", host)
		}
	}
	for _, asset := range []string{"/swagger/assets/swagger-ui.css", "/swagger/assets/swagger-ui-bundle.js"} {
		if !strings.Contains(body, asset) {
			t.Errorf("the Swagger page does not reference %s", asset)
		}
		request := httptest.NewRequest(http.MethodGet, asset, nil)
		served := httptest.NewRecorder()
		GetSwaggerAsset(served, request)
		if served.Code != http.StatusOK || served.Body.Len() == 0 {
			t.Errorf("%s: status %d, %d bytes", asset, served.Code, served.Body.Len())
		}
	}
}

func TestSwaggerAssetsStayInsideTheirDirectory(t *testing.T) {
	for _, path := range []string{"/swagger/assets/../openapi.yaml", "/swagger/assets/", "/swagger/assets/nothing.js"} {
		recorder := httptest.NewRecorder()
		GetSwaggerAsset(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, expected 404", path, recorder.Code)
		}
	}
}

// Two documents are served: the supported contract, and the same with the
// interface's own endpoints.
func TestBothAPIDocumentsAreServed(t *testing.T) {
	for name, handler := range map[string]http.HandlerFunc{
		"/swagger/openapi.yaml":        GetOpenAPI,
		"/swagger/openapi.public.yaml": GetPublicOpenAPI,
	} {
		recorder := httptest.NewRecorder()
		handler(recorder, httptest.NewRequest(http.MethodGet, name, nil))
		if recorder.Code != http.StatusOK || !strings.HasPrefix(strings.TrimSpace(recorder.Body.String()), "#") && !strings.Contains(recorder.Body.String(), "openapi: 3.0.3") {
			t.Errorf("%s: status %d", name, recorder.Code)
		}
	}
	if len(openAPIPublicSpec) >= len(openAPISpec) {
		t.Error("the supported document is not smaller than the full one; the internal operations were not removed")
	}
}
