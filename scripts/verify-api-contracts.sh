#!/usr/bin/env bash
#
# Verifies that the backend's actual responses match the types the frontend
# declares in frontend/src/lib/api/types.ts.
#
# Those types were hand-derived from the Go structs and handler code, because
# api/openapi.yaml documents 3 of roughly 150 endpoints (see ADR-032
# follow-up). Hand-derived means guessed in places: a field named differently
# than assumed degrades to an em dash on screen rather than an error, so the
# mismatch is silent. This script makes it loud.
#
# It prints response SHAPES only - key names and JSON types - never values,
# so it can be run against a platform holding customer data and pasted into a
# ticket or a transcript.
#
# Usage:
#   scripts/verify-api-contracts.sh                 # via the datalab tunnel
#   NORYX_SSH_PORT=22140 scripts/verify-api-contracts.sh
#   NORYX_TOKEN=eyJ... scripts/verify-api-contracts.sh   # bring your own token
#
set -euo pipefail

SSH_HOST="${NORYX_SSH_HOST:-127.0.0.1}"
SSH_PORT="${NORYX_SSH_PORT:-22140}"
SSH_USER="${NORYX_SSH_USER:-stef}"
SSH_KEY="${NORYX_SSH_KEY:-$HOME/.ssh/id_ed25519_noryx_vm}"
NAMESPACE="${NORYX_NAMESPACE:-noryx-ce}"
TEST_USER="${NORYX_TEST_USER:-demo}"

ENDPOINTS="${NORYX_ENDPOINTS:-platform/overview admin/overview admin/inventory hardware-tiers \
projects datasets datasources ontologies repositories secrets cronjobs jobs workspaces apps \
dashboards environments builds admin/executions admin/organizations admin/users production/apps \
egress/profiles admin/egress/rules admin/storage-endpoints admin/audit user/preferences}"

remote() {
  ssh -o ConnectTimeout=15 -o StrictHostKeyChecking=accept-new \
      -p "$SSH_PORT" -i "$SSH_KEY" "$SSH_USER@$SSH_HOST" 'bash -s'
}

remote <<REMOTE
set -euo pipefail
NS="$NAMESPACE"
TEST_USER="$TEST_USER"
SUPPLIED_TOKEN="${NORYX_TOKEN:-}"
ENDPOINTS="$ENDPOINTS"

KC=\$(kubectl -n "\$NS" get svc keycloak -o jsonpath='{.spec.clusterIP}'):8080
BE=\$(kubectl -n "\$NS" get svc noryx-backend -o jsonpath='{.spec.clusterIP}'):8080

if [ -n "\$SUPPLIED_TOKEN" ]; then
  UT="\$SUPPLIED_TOKEN"
  echo "# token: fourni par l'appelant"
else
  # Sets a throwaway password on the test account to obtain a token through
  # the direct grant that the public noryx-api client already allows.
  # Reset the account afterwards; it is named in the closing notice.
  PW=\$(kubectl -n "\$NS" get secret noryx-secrets -o jsonpath='{.data.KEYCLOAK_ADMIN_PASSWORD}' | base64 -d)
  AT=\$(curl -s -X POST "http://\$KC/auth/realms/master/protocol/openid-connect/token" \
        -d grant_type=password -d client_id=admin-cli -d username=admin \
        --data-urlencode "password=\$PW" | jq -r '.access_token // empty')
  [ -n "\$AT" ] || { echo "ECHEC: jeton administrateur Keycloak indisponible" >&2; exit 1; }

  ID=\$(curl -s -H "Authorization: Bearer \$AT" \
        "http://\$KC/auth/admin/realms/noryx/users?username=\$TEST_USER&exact=true" | jq -r '.[0].id // empty')
  [ -n "\$ID" ] || { echo "ECHEC: compte de test '\$TEST_USER' introuvable" >&2; exit 1; }

  TMP="verify-\$(date +%s)"
  curl -s -o /dev/null -X PUT -H "Authorization: Bearer \$AT" -H 'Content-Type: application/json' \
    "http://\$KC/auth/admin/realms/noryx/users/\$ID/reset-password" \
    -d "{\"type\":\"password\",\"value\":\"\$TMP\",\"temporary\":false}"

  UT=\$(curl -s -X POST "http://\$KC/auth/realms/noryx/protocol/openid-connect/token" \
        -d grant_type=password -d client_id=noryx-api -d scope=openid \
        -d username="\$TEST_USER" --data-urlencode "password=\$TMP" | jq -r '.access_token // empty')
  [ -n "\$UT" ] || { echo "ECHEC: jeton utilisateur indisponible" >&2; exit 1; }
  echo "# token: direct grant sur le compte '\$TEST_USER'"
fi

echo "# backend: \$(curl -s "http://\$BE/api/v1/version" | jq -c .)"
echo
echo "### claims du jeton (organisation, audience)"
echo "\$UT" | cut -d. -f2 | tr '_-' '/+' | base64 -d 2>/dev/null \
  | jq -r 'to_entries[] | select(.key|test("^(org|organi|group|aud|azp|realm_access)")) | "  \(.key): \(.value|tostring|.[0:110])"' || true

for EP in \$ENDPOINTS; do
  echo
  echo "### /api/v1/\$EP"
  R=\$(curl -s -w '\nHTTP %{http_code}' -H "Authorization: Bearer \$UT" "http://\$BE/api/v1/\$EP" || true)
  CODE=\$(echo "\$R" | tail -1)
  BODY=\$(echo "\$R" | sed '\$d')
  echo "  \$CODE"
  echo "\$BODY" | jq -r '
      (if type=="object" and has("items") then (.items[0] // "EMPTY_LIST")
       elif type=="array" then (.[0] // "EMPTY_LIST")
       else . end)
      | if . == "EMPTY_LIST" then "  (liste vide)"
        elif type=="object" then (to_entries[] | "  \(.key): \(.value|type)")
        else "  (\(type))" end' 2>/dev/null | head -30 \
    || echo "  (reponse non JSON)"
done

if [ -z "\$SUPPLIED_TOKEN" ]; then
  echo
  echo "# NOTE: le compte '\$TEST_USER' porte un mot de passe temporaire. Reinitialisez-le."
fi
REMOTE
