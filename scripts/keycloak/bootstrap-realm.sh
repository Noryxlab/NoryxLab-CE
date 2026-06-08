#!/usr/bin/env bash
set -euo pipefail

NS="${NS:-noryx-ce}"
REALM="${REALM:-noryx}"
ADMIN_USER="${ADMIN_USER:-admin}"
ADMIN_PASS="${ADMIN_PASS:-change-me}"
BOOTSTRAP_USER="${BOOTSTRAP_USER:-stef}"
BOOTSTRAP_PASS="${BOOTSTRAP_PASS:-change-me-stef}"
BOOTSTRAP_EMAIL="${BOOTSTRAP_EMAIL:-stef@noryxlab.ai}"
API_CLIENT_ID="${API_CLIENT_ID:-noryx-api}"

pod="$(kubectl -n "$NS" get pod -l app=keycloak -o jsonpath='{.items[0].metadata.name}')"

exec_kc() {
  kubectl -n "$NS" exec "$pod" -- /opt/keycloak/bin/kcadm.sh "$@"
}

exec_kc config credentials --server http://127.0.0.1:8080/auth --realm master --user "$ADMIN_USER" --password "$ADMIN_PASS" >/dev/null

if ! exec_kc get "realms/$REALM" >/dev/null 2>&1; then
  exec_kc create realms -s realm="$REALM" -s enabled=true >/dev/null
fi

if ! exec_kc get "realms/$REALM/roles/noryx-admin" >/dev/null 2>&1; then
  exec_kc create "realms/$REALM/roles" -s name=noryx-admin >/dev/null
fi

if ! exec_kc get "realms/$REALM/clients?clientId=$API_CLIENT_ID" | grep -q '"clientId"'; then
  exec_kc create "realms/$REALM/clients" \
    -s clientId="$API_CLIENT_ID" \
    -s publicClient=true \
    -s standardFlowEnabled=true \
    -s directAccessGrantsEnabled=true \
    -s 'redirectUris=["*"]' \
    -s 'webOrigins=["*"]' >/dev/null
fi

if ! exec_kc get "realms/$REALM/users?username=$BOOTSTRAP_USER" | grep -q '"username"'; then
  exec_kc create "realms/$REALM/users" \
    -s username="$BOOTSTRAP_USER" \
    -s enabled=true \
    -s email="$BOOTSTRAP_EMAIL" \
    -s emailVerified=true >/dev/null
fi

exec_kc set-password -r "$REALM" --username "$BOOTSTRAP_USER" --new-password "$BOOTSTRAP_PASS" --temporary=false >/dev/null
exec_kc add-roles -r "$REALM" --uusername "$BOOTSTRAP_USER" --rolename noryx-admin >/dev/null

echo "Realm $REALM ready. User $BOOTSTRAP_USER is noryx-admin."
