#!/usr/bin/env bash
#
# Creates a minimal set of objects on a Noryx test platform so that the
# collection endpoints return something, and scripts/verify-api-contracts.sh
# can observe their shape.
#
# A read-only survey cannot reach these: on an empty platform every collection
# is an empty list, and no creation path is exercised at all. That is how three
# broken creation payloads survived review.
#
# Creates a project, a dataset, a workspace, a job and an app. Idempotent by
# name: re-running reuses whatever already exists.
#
# TEST PLATFORMS ONLY. It launches real workloads and consumes cluster
# capacity.
#
set -euo pipefail

NS="${NORYX_NAMESPACE:-noryx-ce}"
REALM="${NORYX_REALM:-noryx}"
OPS_SECRET="${NORYX_OPS_SECRET:-noryx-ops-account}"
BE_PORT="${NORYX_BE_PORT:-18090}"
KC_PORT="${NORYX_KC_PORT:-18091}"
PREFIX="${NORYX_SEED_PREFIX:-verif}"

command -v kubectl >/dev/null || { echo "kubectl introuvable" >&2; exit 1; }

PIDS=()
cleanup() { for p in "${PIDS[@]:-}"; do kill "$p" 2>/dev/null || true; done; }
trap cleanup EXIT

kubectl -n "$NS" port-forward svc/noryx-backend "$BE_PORT:8080" >/dev/null 2>&1 & PIDS+=($!)
kubectl -n "$NS" port-forward svc/keycloak "$KC_PORT:8080" >/dev/null 2>&1 & PIDS+=($!)
for _ in $(seq 40); do curl -sf "http://127.0.0.1:$BE_PORT/healthz" >/dev/null 2>&1 && break; sleep 0.25; done
curl -sf "http://127.0.0.1:$BE_PORT/healthz" >/dev/null || { echo "backend injoignable" >&2; exit 1; }

U="$(kubectl -n "$NS" get secret "$OPS_SECRET" -o jsonpath='{.data.USERNAME}' | base64 -d)"
P="$(kubectl -n "$NS" get secret "$OPS_SECRET" -o jsonpath='{.data.PASSWORD}' | base64 -d)"
TOKEN="$(curl -s -X POST "http://127.0.0.1:$KC_PORT/auth/realms/$REALM/protocol/openid-connect/token" \
  -d grant_type=password -d client_id=noryx-api -d scope=openid \
  -d username="$U" --data-urlencode "password=$P" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin).get("access_token",""))')"
[ -n "$TOKEN" ] || { echo "jeton indisponible; lancez scripts/provision-ops-account.sh" >&2; exit 1; }

API="http://127.0.0.1:$BE_PORT/api/v1"
AUTH="Authorization: Bearer $TOKEN"
JSON='Content-Type: application/json'

get_id() { python3 -c 'import json,sys; print(json.load(sys.stdin).get("id",""))' 2>/dev/null || true; }

find_by_name() { # $1 collection, $2 name
  curl -s -H "$AUTH" "$API/$1" | python3 -c '
import json,sys
name=sys.argv[1]
d=json.load(sys.stdin)
items=d.get("items") or [] if isinstance(d,dict) else d
print(next((i["id"] for i in items if i.get("name")==name), ""))
' "$2" 2>/dev/null || true
}

post() { # $1 path, $2 body -> prints id, reports failures
  local out code
  out="$(curl -s -w '\n%{http_code}' -H "$AUTH" -H "$JSON" -d "$2" "$API/$1")"
  code="$(printf '%s' "$out" | tail -1)"
  if [ "$code" != "200" ] && [ "$code" != "201" ] && [ "$code" != "202" ]; then
    echo "  ECHEC $1 -> HTTP $code : $(printf '%s' "$out" | sed '$d' | head -c 200)" >&2
    return 1
  fi
  printf '%s' "$out" | sed '$d' | get_id
}

# --- projet -----------------------------------------------------------------
PROJECT_NAME="$PREFIX-projet"
PROJECT_ID="$(find_by_name projects "$PROJECT_NAME")"
if [ -z "$PROJECT_ID" ]; then
  PROJECT_ID="$(post projects "{\"name\":\"$PROJECT_NAME\",\"description\":\"Jeu de test pour la verification des contrats d API\"}")"
  echo "projet cree: $PROJECT_ID"
else
  echo "projet existant: $PROJECT_ID"
fi

# --- dataset ----------------------------------------------------------------
DATASET_NAME="$PREFIX-dataset"
DATASET_ID="$(find_by_name datasets "$DATASET_NAME")"
if [ -z "$DATASET_ID" ]; then
  DATASET_ID="$(post datasets "{\"name\":\"$DATASET_NAME\",\"description\":\"Dataset local de verification\",\"classification\":\"non-hds\"}")" \
    && echo "dataset cree: $DATASET_ID"
else
  echo "dataset existant: $DATASET_ID"
fi
[ -n "${DATASET_ID:-}" ] && curl -s -o /dev/null -X PUT -H "$AUTH" \
  "$API/projects/$PROJECT_ID/datasets/$DATASET_ID" && echo "dataset rattache au projet"

# Image and ide must come from the *same* environment, otherwise the pair is
# inconsistent: the first environment with an image is not necessarily the
# first one that also declares a workspace ide.
ENV_PAIR="$(curl -s -H "$AUTH" "$API/environments" | python3 -c '
import json,sys
d=json.load(sys.stdin); items=d.get("items") or []
for e in items:
    if e.get("destinationImage") and e.get("workspaceIdes"):
        print(e["destinationImage"], e["workspaceIdes"][0]); break
else:
    print("", "vscode")
' 2>/dev/null || echo " vscode")"
IMAGE="$(printf '%s' "$ENV_PAIR" | cut -d' ' -f1)"
IDE="$(printf '%s' "$ENV_PAIR" | cut -d' ' -f2)"
echo "image retenue: ${IMAGE:-<defaut>} (ide=$IDE)"

# --- workspace --------------------------------------------------------------
WS_NAME="$PREFIX-workspace"
if [ -z "$(find_by_name workspaces "$WS_NAME")" ]; then
  post workspaces "{\"projectId\":\"$PROJECT_ID\",\"name\":\"$WS_NAME\",\"ide\":\"$IDE\"$([ -n "$IMAGE" ] && echo ",\"image\":\"$IMAGE\"")}" \
    >/dev/null && echo "workspace lance"
else
  echo "workspace existant"
fi

# --- job --------------------------------------------------------------------
if [ -n "$IMAGE" ]; then
  post jobs "{\"projectId\":\"$PROJECT_ID\",\"name\":\"$PREFIX-job\",\"image\":\"$IMAGE\",\"command\":[\"python\",\"-c\",\"print(1)\"]}" \
    >/dev/null && echo "job lance"
else
  echo "job ignore: aucune image d environnement disponible"
fi

# --- application ------------------------------------------------------------
if [ -n "$IMAGE" ]; then
  post apps "{\"projectId\":\"$PROJECT_ID\",\"name\":\"$PREFIX-app\",\"slug\":\"$PREFIX-app\",\"image\":\"$IMAGE\",\"command\":[\"python3\",\"-m\",\"http.server\",\"9000\"],\"port\":9000,\"accessMode\":\"private\"}" \
    >/dev/null && echo "application creee"
else
  echo "application ignoree: aucune image d environnement disponible"
fi

# --- planification ----------------------------------------------------------
if [ -n "$IMAGE" ]; then
  post cronjobs "{\"projectId\":\"$PROJECT_ID\",\"name\":\"$PREFIX-cron\",\"image\":\"$IMAGE\",\"command\":[\"python\",\"-c\",\"print(1)\"],\"schedule\":\"0 8 * * 1-5\",\"timeZone\":\"Europe/Paris\"}" \
    >/dev/null && echo "planification creee"
fi

# --- dashboard --------------------------------------------------------------
if [ -n "$IMAGE" ]; then
  post dashboards "{\"projectId\":\"$PROJECT_ID\",\"name\":\"$PREFIX-dashboard\",\"slug\":\"$PREFIX-dashboard\",\"image\":\"$IMAGE\",\"command\":[\"python3\",\"-m\",\"http.server\",\"9000\"],\"port\":9000,\"accessMode\":\"private\"}" \
    >/dev/null && echo "dashboard cree"
fi

echo
echo "Termine. Lancez scripts/verify-api-contracts.sh pour relever les formes."
