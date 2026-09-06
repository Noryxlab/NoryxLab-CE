package http

import (
	"embed"
	_ "embed"
	"net/http"
	"strings"
)

//go:embed static/openapi.yaml
var openAPISpec []byte

// The same document with the operations marked `x-noryx-internal` removed:
// what an integration may build on, without the interface's own aggregations.
//
//go:embed static/openapi.public.yaml
var openAPIPublicSpec []byte

// Swagger UI itself, served from the platform.
//
// It used to be loaded from unpkg.com, which works on a laptop with internet
// access and nowhere else: on an installation behind a VPN with no outbound
// route - which is both production installations - the page loaded, stayed
// blank, and said nothing. The specification was fine; the reader had no way
// to see it.
//
//go:embed static/swagger-ui
var swaggerUIAssets embed.FS

const swaggerUIHTML = `<!doctype html>
<html>
  <head>
    <meta charset="utf-8" />
    <title>Noryx API</title>
    <link rel="stylesheet" href="/swagger/assets/swagger-ui.css" />
  </head>
  <body>
    <div id="swagger-ui"></div>
    <script src="/swagger/assets/swagger-ui-bundle.js"></script>
    <script>
      window.ui = SwaggerUIBundle({
        urls: [
          { name: 'Noryx API (supported)', url: '/swagger/openapi.public.yaml' },
          { name: 'Noryx API including internal endpoints', url: '/swagger/openapi.yaml' }
        ],
        dom_id: '#swagger-ui'
      });
    </script>
  </body>
</html>
`

func GetOpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(openAPISpec)
}

func GetPublicOpenAPI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/yaml")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(openAPIPublicSpec)
}

func GetSwaggerUI(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(swaggerUIHTML))
}

func GetSwaggerAsset(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/swagger/assets/")
	if name == "" || strings.Contains(name, "/") {
		http.NotFound(w, r)
		return
	}
	body, err := swaggerUIAssets.ReadFile("static/swagger-ui/" + name)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	switch {
	case strings.HasSuffix(name, ".css"):
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
	case strings.HasSuffix(name, ".js"):
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	}
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}
