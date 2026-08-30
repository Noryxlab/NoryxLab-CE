#!/usr/bin/env bash
#
# Provisions a dedicated operations account for automated verification against
# a Noryx test platform.
#
# Why a real user rather than a client-credentials service account: Noryx
# resolves authorisation from the user's organisation membership, enforced
# server-side (`organization_required`). A client_credentials token carries no
# organisation claim, so it is refused by every endpoint. This account is
# therefore a technical *user*, member of a dedicated organisation.
#
# Not to be confused with the confidential `noryx-backend` client recorded in
# the ADR-033 follow-up: that one replaces the Keycloak master administrator
# for backend-to-Keycloak calls. This one is for API-to-Noryx calls.
#
# Idempotent: safe to re-run. Re-running rotates the password.
#
# Usage:
#   scripts/provision-ops-account.sh
#   NORYX_OPS_ADMIN=0 scripts/provision-ops-account.sh   # without the admin role
#
set -euo pipefail

NS="${NORYX_NAMESPACE:-noryx-ce}"
REALM="${NORYX_REALM:-noryx}"
OPS_USER="${NORYX_OPS_USER:-noryxops}"
OPS_ORG_NAME="${NORYX_OPS_ORG_NAME:-Noryx Ops}"
OPS_ORG_ALIAS="${NORYX_OPS_ORG_ALIAS:-noryxops}"
OPS_SECRET="${NORYX_OPS_SECRET:-noryx-ops-account}"
GRANT_ADMIN="${NORYX_OPS_ADMIN:-1}"
ADMIN_ROLE="noryx-admin"

command -v kubectl >/dev/null || { echo "kubectl introuvable" >&2; exit 1; }

# Generated locally so it can be stored in the cluster secret without ever
# being echoed to the terminal.
PASSWORD="$(openssl rand -hex 32)"

POD="$(kubectl -n "$NS" get pod -l app=keycloak -o jsonpath='{.items[0].metadata.name}')"
[ -n "$POD" ] || { echo "pod keycloak introuvable dans $NS" >&2; exit 1; }

echo "namespace=$NS realm=$REALM utilisateur=$OPS_USER organisation=$OPS_ORG_ALIAS admin=$GRANT_ADMIN"

kubectl -n "$NS" exec -i "$POD" -- bash -s -- \
  "$REALM" "$OPS_USER" "$OPS_ORG_NAME" "$OPS_ORG_ALIAS" "$PASSWORD" "$GRANT_ADMIN" "$ADMIN_ROLE" <<'REMOTE'
set -euo pipefail
REALM="$1"; OPS_USER="$2"; ORG_NAME="$3"; ORG_ALIAS="$4"; PASSWORD="$5"; GRANT_ADMIN="$6"; ADMIN_ROLE="$7"
KC=/opt/keycloak/bin/kcadm.sh
CFG=/tmp/kcadm-ops-account.config

"$KC" config credentials --config "$CFG" \
  --server http://127.0.0.1:8080/auth --realm master \
  --user "$KC_BOOTSTRAP_ADMIN_USERNAME" --password "$KC_BOOTSTRAP_ADMIN_PASSWORD" >/dev/null

id_of() { sed -n 's/.*"id" : "\([^"]*\)".*/\1/p' | sed -n '1p'; }

# --- organisation -----------------------------------------------------------
ORG_ID="$("$KC" get "organizations?search=$ORG_ALIAS" -r "$REALM" --config "$CFG" --fields id,alias | id_of)"
if [ -z "$ORG_ID" ]; then
  ORG_ID="$("$KC" create organizations -r "$REALM" --config "$CFG" \
    -s name="$ORG_NAME" -s alias="$ORG_ALIAS" -s enabled=true -i)"
  echo "organisation creee: $ORG_ALIAS"
else
  echo "organisation existante: $ORG_ALIAS"
fi

# --- utilisateur ------------------------------------------------------------
USER_ID="$("$KC" get "users?username=$OPS_USER&exact=true" -r "$REALM" --config "$CFG" --fields id | id_of)"
if [ -z "$USER_ID" ]; then
  USER_ID="$("$KC" create users -r "$REALM" --config "$CFG" \
    -s username="$OPS_USER" -s enabled=true \
    -s email="$OPS_USER@noryx.local" -s emailVerified=true \
    -s firstName=Noryx -s lastName=Ops -i)"
  echo "utilisateur cree: $OPS_USER"
else
  echo "utilisateur existant: $OPS_USER"
fi

"$KC" set-password -r "$REALM" --config "$CFG" --userid "$USER_ID" --new-password "$PASSWORD" >/dev/null
echo "mot de passe defini"

# --- appartenance a l'organisation ------------------------------------------
if "$KC" get "organizations/$ORG_ID/members" -r "$REALM" --config "$CFG" --fields id \
     | grep -q "\"id\" : \"$USER_ID\""; then
  echo "deja membre de l'organisation"
else
  "$KC" create "organizations/$ORG_ID/members" -r "$REALM" --config "$CFG" -b "\"$USER_ID\"" >/dev/null
  echo "ajoute a l'organisation"
fi

# --- role d'administration globale ------------------------------------------
if [ "$GRANT_ADMIN" = "1" ]; then
  "$KC" get "roles/$ADMIN_ROLE" -r "$REALM" --config "$CFG" >/dev/null 2>&1 \
    || "$KC" create roles -r "$REALM" --config "$CFG" -s name="$ADMIN_ROLE" \
         -s description="Administration globale de la plateforme Noryx" >/dev/null
  "$KC" add-roles -r "$REALM" --config "$CFG" --uusername "$OPS_USER" --rolename "$ADMIN_ROLE" >/dev/null
  echo "role $ADMIN_ROLE attribue"
else
  echo "role $ADMIN_ROLE non attribue (NORYX_OPS_ADMIN=0)"
fi
REMOTE

# The password lives in the cluster rather than on disk, so the verification
# script can read it without a credential ever passing through a terminal.
kubectl -n "$NS" create secret generic "$OPS_SECRET" \
  --from-literal=USERNAME="$OPS_USER" \
  --from-literal=PASSWORD="$PASSWORD" \
  --dry-run=client -o yaml | kubectl -n "$NS" apply -f - >/dev/null
echo "identifiants stockes dans le secret $NS/$OPS_SECRET"

echo
echo "Termine. Lancez scripts/verify-api-contracts.sh pour verifier l'acces."
