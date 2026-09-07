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

// The page has to hand Swagger UI a document it will actually load. Passing a
// list of them to SwaggerUIBundle without the standalone preset renders "No
// API definition provided" on a blank page, which is what the first reader of
// this page got.
func TestTheSwaggerPageNamesOneDocumentToLoad(t *testing.T) {
	for _, testCase := range []struct {
		query, expected, other string
	}{
		{"/swagger", "/swagger/openapi.public.yaml", "url: '/swagger/openapi.yaml'"},
		{"/swagger?spec=full", "/swagger/openapi.yaml", "url: '/swagger/openapi.public.yaml'"},
	} {
		recorder := httptest.NewRecorder()
		GetSwaggerUI(recorder, httptest.NewRequest(http.MethodGet, testCase.query, nil))
		body := recorder.Body.String()
		if !strings.Contains(body, "url: '"+testCase.expected+"'") {
			t.Errorf("%s does not load %s", testCase.query, testCase.expected)
		}
		if strings.Contains(body, testCase.other) {
			t.Errorf("%s loads the other document as well", testCase.query)
		}
		if strings.Contains(body, "urls:") {
			t.Errorf("%s passes a list of documents, which this page's setup ignores", testCase.query)
		}
		if strings.Contains(body, "SPEC_URL") || strings.Contains(body, "SPEC_NOTE") {
			t.Errorf("%s left a placeholder in the page", testCase.query)
		}
		// Both documents stay reachable from the page, whichever is loaded.
		for _, link := range []string{`href="/swagger"`, `href="/swagger?spec=full"`} {
			if !strings.Contains(body, link) {
				t.Errorf("%s does not offer %s", testCase.query, link)
			}
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
