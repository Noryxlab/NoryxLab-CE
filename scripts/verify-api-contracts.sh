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
# Credentials: uses the operations account provisioned by
# scripts/provision-ops-account.sh, whose password lives in a cluster secret.
# Nothing is mutated. Set NORYX_TOKEN to supply your own bearer instead.
#
# Usage:
#   scripts/verify-api-contracts.sh
#   NORYX_TOKEN=eyJ... scripts/verify-api-contracts.sh
#
set -euo pipefail

NS="${NORYX_NAMESPACE:-noryx-ce}"
REALM="${NORYX_REALM:-noryx}"
OPS_SECRET="${NORYX_OPS_SECRET:-noryx-ops-account}"

ENDPOINTS="${NORYX_ENDPOINTS:-version platform/overview admin/overview admin/inventory \
hardware-tiers projects datasets datasources ontologies repositories secrets cronjobs jobs \
workspaces apps dashboards environments builds production/apps egress/profiles \
admin/executions admin/organizations admin/users admin/audit admin/data-usage \
admin/rbac-matrix admin/storage-endpoints admin/egress/rules user/preferences}"

command -v kubectl >/dev/null || { echo "kubectl introuvable" >&2; exit 1; }

POD="$(kubectl -n "$NS" get pod -l app=keycloak -o jsonpath='{.items[0].metadata.name}')"
KC_IP="$(kubectl -n "$NS" get svc keycloak -o jsonpath='{.spec.clusterIP}')"
BE_IP="$(kubectl -n "$NS" get svc noryx-backend -o jsonpath='{.spec.clusterIP}')"
[ -n "$POD" ] || { echo "pod keycloak introuvable" >&2; exit 1; }

# The survey runs inside the cluster because the backend is a ClusterIP
# service, unreachable from the workstation through the API tunnel.
kubectl -n "$NS" exec -i "$POD" -- bash -s -- \
  "$REALM" "$KC_IP" "$BE_IP" "${NORYX_TOKEN:-}" \
  "$(kubectl -n "$NS" get secret "$OPS_SECRET" -o jsonpath='{.data.USERNAME}' 2>/dev/null || true)" \
  "$(kubectl -n "$NS" get secret "$OPS_SECRET" -o jsonpath='{.data.PASSWORD}' 2>/dev/null || true)" \
  "$ENDPOINTS" <<'REMOTE'
set -uo pipefail
REALM="$1"; KC_IP="$2"; BE_IP="$3"; SUPPLIED="$4"; B64_USER="$5"; B64_PASS="$6"; ENDPOINTS="$7"

if [ -n "$SUPPLIED" ]; then
  TOKEN="$SUPPLIED"
  echo "# jeton: fourni par l'appelant"
else
  [ -n "$B64_USER" ] && [ -n "$B64_PASS" ] || {
    echo "ECHEC: secret du compte ops absent. Lancez scripts/provision-ops-account.sh" >&2; exit 1; }
  U="$(printf '%s' "$B64_USER" | base64 -d)"
  P="$(printf '%s' "$B64_PASS" | base64 -d)"
  TOKEN="$(curl -s -X POST "http://$KC_IP:8080/auth/realms/$REALM/protocol/openid-connect/token" \
    -d grant_type=password -d client_id=noryx-api -d scope=openid \
    -d username="$U" --data-urlencode "password=$P" \
    | sed -n 's/.*"access_token":"\([^"]*\)".*/\1/p')"
  [ -n "$TOKEN" ] || { echo "ECHEC: jeton indisponible pour le compte ops" >&2; exit 1; }
  echo "# jeton: compte de service $U"
fi

echo "# backend: $(curl -s "http://$BE_IP:8080/api/v1/version")"
echo
echo "### claims du jeton"
printf '%s' "$TOKEN" | cut -d. -f2 | tr '_-' '/+' \
  | { read -r p; printf '%s' "$p$(printf '=%.0s' $(seq $(( (4 - ${#p} % 4) % 4 )) ) )"; } \
  | base64 -d 2>/dev/null \
  | tr ',' '\n' | grep -oE '"(organizations|aud|azp|preferred_username)"[^,]*' | head -6 || true

for EP in $ENDPOINTS; do
  echo
  echo "### /api/v1/$EP"
  R="$(curl -s -w '\n%{http_code}' -H "Authorization: Bearer $TOKEN" "http://$BE_IP:8080/api/v1/$EP")"
  CODE="$(printf '%s' "$R" | tail -1)"
  BODY="$(printf '%s' "$R" | sed '$d')"
  echo "  HTTP $CODE"
  # Shape only: first element of the collection, keys and JSON types.
  printf '%s' "$BODY" | python3 -c '
import json,sys
try: d=json.load(sys.stdin)
except Exception: print("  (reponse non JSON)"); raise SystemExit
if isinstance(d,dict) and "items" in d:
    d = (d["items"] or [None])[0]
    if d is None: print("  (liste vide)"); raise SystemExit
elif isinstance(d,list):
    if not d: print("  (liste vide)"); raise SystemExit
    d = d[0]
if isinstance(d,dict):
    for k,v in d.items():
        t=type(v).__name__
        t={"str":"string","int":"number","float":"number","bool":"boolean","NoneType":"null","dict":"object","list":"array"}.get(t,t)
        print(f"  {k}: {t}")
else:
    print(f"  ({type(d).__name__})")
' 2>/dev/null || echo "  (analyse impossible)"
done
REMOTE
