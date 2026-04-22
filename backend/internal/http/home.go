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
      body { margin: 0; min-height: 100vh; background: #ffffff; color: #111111; }
      .wrap { max-width: 980px; margin: 48px auto; padding: 0 16px; }
      h1 { margin: 0 0 8px; font-size: 42px; letter-spacing: 0.02em; }
      p { margin: 0 0 16px; color: #4b5563; }
      .row { display: flex; gap: 10px; flex-wrap: wrap; margin: 12px 0 16px; }
      button { border: 1px solid #d1d5db; background: #f9fafb; color: #111827; border-radius: 8px; padding: 8px 12px; cursor: pointer; }
      button.primary { background: #111827; color: #fff; border-color: #111827; }
      button:disabled { opacity: 0.5; cursor: default; }
      .meta { font-size: 14px; color: #374151; margin-bottom: 16px; }
      .cards { display: grid; grid-template-columns: 1fr; gap: 14px; }
      .card { border: 1px solid #e5e7eb; border-radius: 12px; padding: 12px; }
      h3 { margin: 0 0 8px; font-size: 16px; }
      pre { margin: 0; background: #0b1220; color: #dbeafe; border-radius: 10px; padding: 10px; overflow: auto; min-height: 120px; font-size: 12px; }
      a { color: #0f172a; text-underline-offset: 3px; }
    </style>
    <script src="/auth/js/keycloak.js"></script>
  </head>
  <body>
    <main class="wrap">
      <h1>NoryxLab</h1>
      <p>Community Edition - minimal front login</p>
      <div class="row">
        <button id="login" class="primary">Login Keycloak</button>
        <button id="logout">Logout</button>
        <button id="loadUsers">Load Users</button>
        <button id="loadModules">Load Modules</button>
        <a href="/swagger">Open API Swagger</a>
      </div>
      <div id="meta" class="meta">Not authenticated</div>
      <section class="cards">
        <div class="card">
          <h3>/api/v1/admin/users</h3>
          <pre id="usersOut">-</pre>
        </div>
        <div class="card">
          <h3>/api/v1/admin/modules</h3>
          <pre id="modulesOut">-</pre>
        </div>
      </section>
    </main>
    <script>
      const keycloak = new Keycloak({
        url: window.location.origin + '/auth',
        realm: 'noryx',
        clientId: 'noryx-api'
      });

      const meta = document.getElementById('meta');
      const usersOut = document.getElementById('usersOut');
      const modulesOut = document.getElementById('modulesOut');
      const loginBtn = document.getElementById('login');
      const logoutBtn = document.getElementById('logout');
      const usersBtn = document.getElementById('loadUsers');
      const modulesBtn = document.getElementById('loadModules');

      function setMeta(text) { meta.textContent = text; }
      function pretty(node, data) { node.textContent = JSON.stringify(data, null, 2); }
      function authHeader() { return { Authorization: 'Bearer ' + keycloak.token }; }

      function setButtons(enabled) {
        logoutBtn.disabled = !enabled;
        usersBtn.disabled = !enabled;
        modulesBtn.disabled = !enabled;
      }

      async function callJSON(url) {
        const res = await fetch(url, { headers: authHeader() });
        const body = await res.json().catch(() => ({}));
        if (!res.ok) throw body;
        return body;
      }

      loginBtn.onclick = () => keycloak.login();
      logoutBtn.onclick = () => keycloak.logout({ redirectUri: window.location.origin + '/' });

      usersBtn.onclick = async () => {
        try { pretty(usersOut, await callJSON('/api/v1/admin/users')); }
        catch (e) { pretty(usersOut, e); }
      };

      modulesBtn.onclick = async () => {
        try { pretty(modulesOut, await callJSON('/api/v1/admin/modules')); }
        catch (e) { pretty(modulesOut, e); }
      };

      keycloak.init({ onLoad: 'check-sso', pkceMethod: 'S256', checkLoginIframe: false }).then((authenticated) => {
        if (!authenticated) {
          setMeta('Not authenticated');
          setButtons(false);
          return;
        }
        setMeta('Authenticated as ' + (keycloak.tokenParsed?.preferred_username || keycloak.tokenParsed?.email || keycloak.tokenParsed?.sub));
        setButtons(true);
      }).catch((err) => {
        setMeta('Keycloak init error');
        pretty(usersOut, { error: String(err) });
        setButtons(false);
      });
    </script>
  </body>
</html>
`

func GetHome(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(homeHTML))
}
