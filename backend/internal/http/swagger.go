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

// The page picks its document from the query string rather than handing
// Swagger UI a list of them. `urls` is only read by the standalone preset's
// topbar, which this page does not load: passing it to SwaggerUIBundle alone
// makes it render "No API definition provided" on a blank page - which is what
// the first reader of this page got. A link that reloads with ?spec=full needs
// nothing but the bundle, and says in our words what the two documents are.
const swaggerUIHTML = `<!doctype html>
<html>
  <head>
    <meta charset="utf-8" />
    <title>Noryx API</title>
    <link rel="stylesheet" href="/swagger/assets/swagger-ui.css" />
    <style>
      body { margin: 0; }
      .noryx-bar {
        display: flex; align-items: baseline; gap: 16px; flex-wrap: wrap;
        padding: 14px 20px; background: #1b1b26; color: #d7dce5;
        font: 400 13px/1.5 system-ui, -apple-system, sans-serif;
      }
      .noryx-bar b { color: #fff; font-weight: 600; }
      .noryx-bar span { color: #98a2b3; }
      .noryx-bar a { color: #7cc0ff; text-decoration: none; }
      .noryx-bar a:hover { text-decoration: underline; }
      .noryx-bar a[aria-current] { color: #fff; font-weight: 600; text-decoration: none; }
    </style>
  </head>
  <body>
    <div class="noryx-bar">
      <b>Noryx API</b>
      <a href="/swagger" SUPPORTED_CURRENT>Supported API</a>
      <a href="/swagger?spec=full" FULL_CURRENT>Including internal endpoints</a>
      <span>SPEC_NOTE</span>
    </div>
    <div id="swagger-ui"></div>
    <script src="/swagger/assets/swagger-ui-bundle.js"></script>
    <script>
      window.ui = SwaggerUIBundle({
        url: 'SPEC_URL',
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

func GetSwaggerUI(w http.ResponseWriter, r *http.Request) {
	full := r.URL.Query().Get("spec") == "full"
	page := swaggerUIHTML
	if full {
		page = strings.NewReplacer(
			"SPEC_URL", "/swagger/openapi.yaml",
			"SUPPORTED_CURRENT", "",
			"FULL_CURRENT", `aria-current="page"`,
			"SPEC_NOTE", "Everything the platform serves. The endpoints marked "+
				"x-noryx-internal are the interface's own and carry no compatibility promise.",
		).Replace(page)
	} else {
		page = strings.NewReplacer(
			"SPEC_URL", "/swagger/openapi.public.yaml",
			"SUPPORTED_CURRENT", `aria-current="page"`,
			"FULL_CURRENT", "",
			"SPEC_NOTE", "The API an integration may build on. Every operation says what it returns.",
		).Replace(page)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(page))
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
