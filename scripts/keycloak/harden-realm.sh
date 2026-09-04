#!/usr/bin/env bash
# Applies the realm settings a production installation needs and that a realm
# created by `bootstrap-realm.sh` does not have.
#
# Three things were open on both production platforms until 2026-09-04:
#
#   - `noryx-api` is a *public* client that had the direct access grant
#     (username + password straight to the token endpoint) enabled, with
#     `redirectUris: ["*"]` and `webOrigins: ["*"]`. A public client with a
#     wildcard redirect hands an authorization code to any address an attacker
#     names, and needs no secret to exchange it.
#   - brute force detection was off, so that endpoint could be tried without
#     limit.
#   - no password policy at all: a one-character password was accepted.
#
# `noryx-api` exists only to be an audience: the interface logs in through
# `noryx-frontend`, and the backend checks the `noryx-api` audience on the
# resulting token. So it needs no flow of its own.
#
# The password policy applies to passwords set from now on. Existing users are
# not locked out and are not forced to change anything.
#
# Note for development: `scripts/verify-api-contracts.sh` and
# `scripts/seed-test-data.sh` obtain tokens through the direct access grant.
# They work against a realm from `bootstrap-realm.sh`; they do not work against
# a realm this script has hardened, which is the point.
set -euo pipefail

NS="${NS:-noryx-ce}"
REALM="${REALM:-noryx}"
API_CLIENT_ID="${API_CLIENT_ID:-noryx-api}"
KUBECTL="${KUBECTL:-kubectl}"
# Passwords already in use are not affected; this is what new ones must meet.
PASSWORD_POLICY="${PASSWORD_POLICY:-length(12) and notUsername and notEmail and passwordHistory(3)}"
FAILURE_FACTOR="${FAILURE_FACTOR:-10}"

pod="$(${KUBECTL} -n "${NS}" get pod -l app=keycloak -o jsonpath='{.items[0].metadata.name}')"

${KUBECTL} -n "${NS}" exec -i "${pod}" -- bash -s -- \
  "${REALM}" "${API_CLIENT_ID}" "${PASSWORD_POLICY}" "${FAILURE_FACTOR}" <<'INNER'
set -euo pipefail
REALM="$1"
API_CLIENT_ID="$2"
PASSWORD_POLICY="$3"
FAILURE_FACTOR="$4"

KC=/opt/keycloak/bin/kcadm.sh
CFG=/tmp/kcadm-harden.config

"$KC" config credentials --config "$CFG" \
  --server http://127.0.0.1:8080/auth \
  --realm master \
  --user "$KC_BOOTSTRAP_ADMIN_USERNAME" \
  --password "$KC_BOOTSTRAP_ADMIN_PASSWORD" >/dev/null

# Lock an account out temporarily rather than permanently: a permanent lockout
# turns a password-guessing attempt against a colleague into a way to remove
# them from the platform until an administrator intervenes.
"$KC" update "realms/$REALM" --config "$CFG" \
  -s bruteForceProtected=true \
  -s permanentLockout=false \
  -s failureFactor="$FAILURE_FACTOR" \
  -s waitIncrementSeconds=60 \
  -s maxFailureWaitSeconds=900 \
  -s quickLoginCheckMilliSeconds=1000 \
  -s minimumQuickLoginWaitSeconds=60 \
  -s "passwordPolicy=$PASSWORD_POLICY" >/dev/null
printf 'Realm %s: brute force detection on (%s attempts), password policy set.\n' "$REALM" "$FAILURE_FACTOR"

# `|| true`: the lookup ends in a grep, which exits 1 when the client is
# absent. Under `set -euo pipefail` that would abort here instead of reporting
# the absence.
api_client_id="$(
  {
    "$KC" get "clients?clientId=$API_CLIENT_ID" -r "$REALM" --config "$CFG" --fields id |
      sed -n 's/.*"id" : "\([^"]*\)".*/\1/p' |
      head -n 1
  } || true
)"

if [[ -z "$api_client_id" ]]; then
  printf 'Client %s does not exist in realm %s; nothing to close.\n' "$API_CLIENT_ID" "$REALM" >&2
  exit 0
fi

"$KC" update "clients/$api_client_id" -r "$REALM" --config "$CFG" \
  -s directAccessGrantsEnabled=false \
  -s standardFlowEnabled=false \
  -s implicitFlowEnabled=false \
  -s 'redirectUris=[]' \
  -s 'webOrigins=[]' >/dev/null
printf 'Client %s is now an audience only: no login flow, no redirect URIs.\n' "$API_CLIENT_ID"
INNER
