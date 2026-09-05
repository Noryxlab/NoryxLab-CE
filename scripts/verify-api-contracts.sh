#!/usr/bin/env bash
#
# Verifies that the backend's actual responses match the types the frontend
# declares in frontend/src/lib/api/types.ts.
#
# Those types were hand-derived from the Go structs and handler code, because
# api/openapi.yaml documents 3 of roughly 150 endpoints (see the ADR-032
# follow-up). Hand-derived means guessed in places, and a field named
# differently than assumed renders as an em dash rather than raising an error,
# so the mismatch is silent. This makes it loud.
#
# Prints response SHAPES only - key names and JSON types - never values, so it
# is safe to run against a platform holding customer data and to paste the
# output into a ticket.
#
# Credentials come from the operations account created by
# scripts/provision-ops-account.sh, whose password lives in a cluster secret.
# Nothing is mutated. Set NORYX_TOKEN to supply your own bearer instead.
#
set -euo pipefail

NS="${NORYX_NAMESPACE:-noryx}"
REALM="${NORYX_REALM:-noryx}"
OPS_SECRET="${NORYX_OPS_SECRET:-noryx-ops-account}"
BE_PORT="${NORYX_BE_PORT:-18090}"
KC_PORT="${NORYX_KC_PORT:-18091}"
DEPTH="${NORYX_DEPTH:-1}"   # 2 pour deplier les objets imbriques
RAW="${NORYX_RAW:-0}"
METHOD="${NORYX_METHOD:-GET}"  # POST pour declencher une operation, usage operationnel      # 1 pour afficher les valeurs, pour un controle operationnel

ENDPOINTS="${NORYX_ENDPOINTS:-version platform/overview admin/overview admin/inventory \
hardware-tiers projects datasets datasources ontologies repositories secrets cronjobs jobs \
workspaces apps dashboards environments builds production/apps egress/profiles \
admin/executions admin/organizations admin/users admin/audit admin/data-usage \
admin/rbac-matrix admin/storage-endpoints admin/egress/rules user/preferences}"

command -v kubectl >/dev/null || { echo "kubectl introuvable" >&2; exit 1; }

# Port-forwarding rather than exec'ing into a pod: the backend and Keycloak
# images are minimal and ship neither curl nor wget.
PIDS=()
cleanup() { for p in "${PIDS[@]:-}"; do kill "$p" 2>/dev/null || true; done; }
trap cleanup EXIT

kubectl -n "$NS" port-forward svc/noryx-backend "$BE_PORT:8080" >/dev/null 2>&1 &
PIDS+=($!)
kubectl -n "$NS" port-forward svc/keycloak "$KC_PORT:8080" >/dev/null 2>&1 &
PIDS+=($!)

for _ in $(seq 40); do
  curl -sf "http://127.0.0.1:$BE_PORT/healthz" >/dev/null 2>&1 && break
  sleep 0.25
done
curl -sf "http://127.0.0.1:$BE_PORT/healthz" >/dev/null || {
  echo "backend injoignable via port-forward" >&2; exit 1; }

if [ -n "${NORYX_TOKEN:-}" ]; then
  TOKEN="$NORYX_TOKEN"
  echo "# jeton: fourni par l'appelant"
else
  U="$(kubectl -n "$NS" get secret "$OPS_SECRET" -o jsonpath='{.data.USERNAME}' | base64 -d)"
  P="$(kubectl -n "$NS" get secret "$OPS_SECRET" -o jsonpath='{.data.PASSWORD}' | base64 -d)"
  TOKEN="$(curl -s -X POST \
    "http://127.0.0.1:$KC_PORT/auth/realms/$REALM/protocol/openid-connect/token" \
    -d grant_type=password -d client_id=noryx-api -d scope=openid \
    -d username="$U" --data-urlencode "password=$P" \
    | python3 -c 'import json,sys; print(json.load(sys.stdin).get("access_token",""))')"
  [ -n "$TOKEN" ] || { echo "ECHEC: jeton indisponible. Lancez scripts/provision-ops-account.sh" >&2; exit 1; }
  echo "# jeton: compte $U"
fi

echo "# backend: $(curl -s "http://127.0.0.1:$BE_PORT/api/v1/version")"
echo
echo "### claims du jeton"
python3 - "$TOKEN" <<'PYEOF'
import base64, json, sys
payload = sys.argv[1].split('.')[1]
payload += '=' * (-len(payload) % 4)
claims = json.loads(base64.urlsafe_b64decode(payload))
for key in ('preferred_username', 'aud', 'azp', 'organizations'):
    if key in claims:
        print(f"  {key}: {json.dumps(claims[key], ensure_ascii=False)[:140]}")
roles = claims.get('realm_access', {}).get('roles', [])
print(f"  realm_roles: {[r for r in roles if not r.startswith('default-')]}")
PYEOF

for EP in $ENDPOINTS; do
  echo
  echo "### /api/v1/$EP"
  BODY="$(curl -s -X "$METHOD" -w '\n%{http_code}' -H "Authorization: Bearer $TOKEN" \
          "http://127.0.0.1:$BE_PORT/api/v1/$EP")"
  echo "  HTTP $(printf '%s' "$BODY" | tail -1)"
  if [ "$RAW" = "1" ]; then
    # Values, not shapes. For operational checks only: the output can contain
    # real data, so do not paste it anywhere without reading it first.
    printf '%s' "$BODY" | sed '$d' | python3 -m json.tool 2>/dev/null | head -40 \
      || printf '%s' "$BODY" | sed '$d' | head -c 600
    continue
  fi
  printf '%s' "$BODY" | sed '$d' | NORYX_DEPTH="$DEPTH" python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
except Exception:
    print("  (reponse non JSON)"); raise SystemExit
if isinstance(d, dict) and "items" in d:
    items = d["items"] or []
    if not items: print("  (liste vide)"); raise SystemExit
    d = items[0]
elif isinstance(d, list):
    if not d: print("  (liste vide)"); raise SystemExit
    d = d[0]
import os
names = {"str":"string","int":"number","float":"number","bool":"boolean",
         "NoneType":"null","dict":"object","list":"array"}
depth = int(os.environ.get("NORYX_DEPTH", "1"))
def tname(v): return names.get(type(v).__name__, type(v).__name__)
def walk(node, indent, level):
    if isinstance(node, dict):
        for k, v in node.items():
            print(f"{indent}{k}: {tname(v)}")
            if level < depth and isinstance(v, dict):
                walk(v, indent + "  ", level + 1)
            elif level < depth and isinstance(v, list) and v and isinstance(v[0], dict):
                print(f"{indent}  [0]:")
                walk(v[0], indent + "    ", level + 1)
    else:
        print(f"{indent}({tname(node)})")
walk(d, "  ", 1)
' 2>/dev/null || echo "  (analyse impossible)"
done
