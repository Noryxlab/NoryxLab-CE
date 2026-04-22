package http

import "net/http"

const homeHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width,initial-scale=1" />
    <title>NoryxLab</title>
    <style>
      :root { font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif; }
      body { margin: 0; min-height: 100vh; display: grid; place-items: center; background: #ffffff; color: #111111; }
      main { text-align: center; }
      h1 { margin: 0 0 12px; font-size: 48px; letter-spacing: 0.02em; }
      p { margin: 0 0 20px; color: #4b5563; }
      a { color: #0f172a; text-underline-offset: 3px; }
    </style>
  </head>
  <body>
    <main>
      <h1>NoryxLab</h1>
      <p>Community Edition bootstrap</p>
      <a href="/swagger">Open API Swagger</a>
    </main>
  </body>
</html>
`

func GetHome(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(homeHTML))
}
