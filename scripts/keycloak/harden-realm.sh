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

NS="${NS:-noryx}"
REALM="${REALM:-noryx}"
API_CLIENT_ID="${API_CLIENT_ID:-noryx-api}"
FRONTEND_CLIENT_ID="${FRONTEND_CLIENT_ID:-noryx-frontend}"
KUBECTL="${KUBECTL:-kubectl}"
# Passwords already in use are not affected; this is what new ones must meet.
PASSWORD_POLICY="${PASSWORD_POLICY:-length(12) and notUsername and notEmail and passwordHistory(3)}"
FAILURE_FACTOR="${FAILURE_FACTOR:-10}"
# How long a session survives with nothing happening, and how long it may last
# at all. Keycloak's default idle timeout is 30 minutes, which is a reasonable
# posture for a bank and a poor one here: "nothing happening" means no request,
# not no human, so reading one screen for half an hour ended the session and
# the next click landed on a sign-in page. Four hours covers a working session;
# the ten-hour ceiling still forces a fresh sign-in every day.
SESSION_IDLE_SECONDS="${SESSION_IDLE_SECONDS:-14400}"
SESSION_MAX_SECONDS="${SESSION_MAX_SECONDS:-36000}"

pod="$(${KUBECTL} -n "${NS}" get pod -l app=keycloak -o jsonpath='{.items[0].metadata.name}')"

${KUBECTL} -n "${NS}" exec -i "${pod}" -- bash -s -- \
  "${REALM}" "${API_CLIENT_ID}" "${PASSWORD_POLICY}" "${FAILURE_FACTOR}" "${FRONTEND_CLIENT_ID}" \
  "${SESSION_IDLE_SECONDS}" "${SESSION_MAX_SECONDS}" <<'INNER'
set -euo pipefail
REALM="$1"
API_CLIENT_ID="$2"
PASSWORD_POLICY="$3"
FAILURE_FACTOR="$4"
FRONTEND_CLIENT_ID="$5"
SESSION_IDLE_SECONDS="$6"
SESSION_MAX_SECONDS="$7"

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
  -s "passwordPolicy=$PASSWORD_POLICY" \
  -s ssoSessionIdleTimeout="$SESSION_IDLE_SECONDS" \
  -s ssoSessionMaxLifespan="$SESSION_MAX_SECONDS" >/dev/null
printf 'Realm %s: brute force detection on (%s attempts), password policy set.\n' "$REALM" "$FAILURE_FACTOR"
printf 'Sessions: %sh idle, %sh maximum.\n' "$((SESSION_IDLE_SECONDS / 3600))" "$((SESSION_MAX_SECONDS / 3600))"


# Keycloak binds its `organization` client scope as *optional*, so the claim
# only appears when a client asks for it by name. The interface asks for
# `openid profile email`, so it never arrived: every user saw a platform where
# they belonged to no organization, while Keycloak held the memberships all
# along. Making it a default scope puts the claim in the token every time.
frontend_client_id="$(
  {
    "$KC" get "clients?clientId=$FRONTEND_CLIENT_ID" -r "$REALM" --config "$CFG" --fields id |
      sed -n 's/.*"id" : "\([^"]*\)".*/\1/p' |
      head -n 1
  } || true
)"
organization_scope_id="$(
  {
    "$KC" get client-scopes -r "$REALM" --config "$CFG" --fields id,name |
      tr -d ' \n' |
      sed -n 's/.*{"id":"\([^"]*\)","name":"organization"}.*/\1/p' |
      head -n 1
  } || true
)"
if [[ -n "$frontend_client_id" && -n "$organization_scope_id" ]]; then
  # A scope cannot be in both lists: Keycloak accepts the second call and
  # silently keeps the first binding, which is how this looked applied while
  # the token still carried nothing. Remove it from the optional list first.
  "$KC" delete "clients/$frontend_client_id/optional-client-scopes/$organization_scope_id" \
    -r "$REALM" --config "$CFG" >/dev/null 2>&1 || true
  "$KC" update "clients/$frontend_client_id/default-client-scopes/$organization_scope_id" \
    -r "$REALM" --config "$CFG" >/dev/null 2>&1 || true
  printf 'Client %s now carries the organization claim by default.\n' "$FRONTEND_CLIENT_ID"
else
  printf 'No organization client scope in realm %s; the interface will show no organizations.\n' "$REALM" >&2
fi

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
