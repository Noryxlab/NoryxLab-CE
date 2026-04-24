# Workspace Troubleshooting (Blank Page / Open Flow)

This runbook targets `datalab.noryxlab.ai` with path-based workspace routing:

- `/workspaces/<workspaceID>/...`
- no wildcard DNS required

## 1) Confirm deployed versions

```bash
curl -sk https://datalab.noryxlab.ai/ | rg "FRONT_VERSION|ce-web"
curl -sk https://datalab.noryxlab.ai/swagger/openapi.yaml | rg "^\\s*version:"
```

Expected for current fix line:

- front `ce-web-0.6.19+`
- back `0.5.21+`

## 2) Check workspace is running

```bash
ssh noryxlab-master 'KUBECONFIG=/home/stef/.kube/config kubectl -n noryx-loads get pods,svc -o wide'
ssh noryxlab-master 'KUBECONFIG=/home/stef/.kube/config kubectl -n noryx-loads get pvc -o wide'
```

If pod/service/PVC are missing, open will fail regardless of UI state.

Longhorn health check:

```bash
ssh noryxlab-master 'KUBECONFIG=/home/stef/.kube/config kubectl -n longhorn-system get pods'
```

## 3) Validate Jupyter HTML from workspace URL

Use the exact `accessUrl` returned by `GET /api/v1/workspaces`:

```bash
curl -sk "<BASE><accessUrl>" | head -n 5
```

Expected: HTML containing `JupyterLab` and `jupyter-config-data`.

## 4) Validate token-to-cookie fallback

First request with URL token should set workspace cookie:

```bash
curl -sk -c /tmp/ws.cookies "<BASE>/workspaces/<workspaceID>/lab?reset&token=<token>" -o /tmp/lab.html -w "%{http_code}\n"
cat /tmp/ws.cookies | rg "noryx_ws_token_<workspaceID>"
```

Then load static asset with only that cookie:

```bash
SCRIPT_PATH=$(rg -o 'src="/workspaces/[^"]+/static/lab/[^"]+\\.js[^"]*"' /tmp/lab.html | head -n1 | sed 's#^src="##; s#"$##')
curl -sk -b /tmp/ws.cookies "<BASE>${SCRIPT_PATH}" -w "%{http_code}\n" -o /tmp/main.js
head -c 40 /tmp/main.js
```

Expected:

- HTTP `200`
- JS payload starts with bundled script content (not JSON error)

## 5) Typical symptoms and causes

- Home page replaced by white page when clicking `Open`
  - front regression opening in same tab
  - fixed in `ce-web-0.6.17+`
- New tab opens but stays blank
  - session/cookie propagation issue in browser
  - validate token-to-cookie flow (step 4)
- `workspace not found` on `/workspaces/<id>/...`
  - in-memory metadata reset after back restart
  - trigger `GET /api/v1/workspaces` authenticated to re-sync runtime records
- `403 insufficient role for workspace access/deletion`
  - caller is not `editor|admin` on workspace project

## 6) Browser-side checks

- hard refresh (`Cmd+Shift+R`)
- allow popups for `datalab.noryxlab.ai`
- check DevTools Network for first failing request under `/workspaces/<workspaceID>/...`
- if first failing response is JSON, read its `error` value and map with sections above
