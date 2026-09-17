#!/usr/bin/env bash
#
# Restores the last backup into a throwaway platform and compares the result.
#
#   ./scripts/ops/rehearse-restore.sh [namespace]
#
# A backup nothing has ever read is a hypothesis. This is what turns it into a
# fact, and it is meant to be run on a schedule rather than after an incident -
# the first time anybody restores must not be the day it matters.
#
# It touches nothing that exists: a separate database on the same server, a
# second backend that reads it, and both removed at the end. The live platform
# is only read from, to compare counts.
set -euo pipefail

NAMESPACE="${1:-${NAMESPACE:-noryx}}"
KUBECTL="${KUBECTL:-kubectl}"
DATABASE="${DATABASE:-noryx_restore_rehearsal}"
IMAGE="${IMAGE:-}"
# The throwaway HTTP client. It is pulled from the public registry by default
# and overridden on an installation that mirrors its own images, which is every
# installation with no route to the internet.
CURL_IMAGE="${CURL_IMAGE:-curlimages/curl:8.10.1}"

say() { printf '  %s\n' "$*"; }
cleanup() {
  ${KUBECTL} -n "${NAMESPACE}" delete deployment,service noryx-restore-rehearsal --ignore-not-found >/dev/null 2>&1 || true
  ${KUBECTL} -n "${NAMESPACE}" delete networkpolicy postgres-allow-restore-rehearsal --ignore-not-found >/dev/null 2>&1 || true
  ${KUBECTL} -n "${NAMESPACE}" exec deployment/postgres -- psql -U noryx -d postgres -c "DROP DATABASE IF EXISTS ${DATABASE}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

if [ -z "${IMAGE}" ]; then
  IMAGE="$(${KUBECTL} -n "${NAMESPACE}" get deployment/noryx-backend -o jsonpath='{.spec.template.spec.containers[0].image}')"
fi
say "rehearsing with ${IMAGE}"

# A credential for the copy, generated per run.
#
# openssl if it is there, /dev/urandom otherwise: this script runs from a
# maintenance shell as often as from CI, and a drill that fails on a missing
# tool is a drill nobody runs.
REHEARSAL_SECRET="$(openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n')"

# This component's own credential, or the shared secret it replaces.
#
# The shared secret was held by every component at once: revoking it for the
# rehearsal broke the validator, its use could not be told apart in an audit,
# and it carried every right when this only needs backups. A component token
# issued with the "operate" scope reaches backups and restores and is refused
# everywhere else.
#
# Read from the cluster when it is not supplied. The token is already there,
# under its own name, and a drill meant to run on a schedule cannot depend on
# somebody remembering to export it - this script could not run at all until
# tonight for exactly that reason.
REHEARSAL_TOKEN="${RESTORE_REHEARSAL_TOKEN:-}"
if [ -z "${REHEARSAL_TOKEN}" ]; then
  REHEARSAL_TOKEN="$(${KUBECTL} -n "${NAMESPACE}" get secret noryx-component-tokens \
    -o jsonpath='{.data.RESTORE_REHEARSAL_TOKEN}' 2>/dev/null | base64 -d 2>/dev/null || true)"
fi
if [ -n "${REHEARSAL_TOKEN}" ]; then
  AUTH_HEADER="Authorization: Bearer ${REHEARSAL_TOKEN}"
elif [ -n "${NORYX_SERVICE_TOKEN:-}" ]; then
  # The shared secret this replaced. Kept for an installation still on it, and
  # nothing more: the platform answers 401 to it since component tokens landed.
  AUTH_HEADER="X-Noryx-Service-Token: ${NORYX_SERVICE_TOKEN}"
else
  # Said here rather than three steps later. Without this the run reached the
  # backup listing, was refused, found no object in the refusal, and reported
  # "no backup object to restore; set OBJECT_KEY" - which sends whoever reads
  # it looking for a missing backup when the backups are fine and the drill has
  # no credential.
  echo "no credential: set RESTORE_REHEARSAL_TOKEN, or make sure the" >&2
  echo "  noryx-component-tokens secret holds RESTORE_REHEARSAL_TOKEN in ${NAMESPACE}" >&2
  exit 1
fi

# The object to restore. Read from the live platform's own record, which is the
# realistic case for a drill; a real recovery passes objectKey by hand.
#
# Asked over the network from a client pod rather than by exec-ing into the
# backend: the backend image is distroless and carries no HTTP client at all,
# so the exec failed, the list came back empty, and the drill stopped with "no
# backup object to restore" - which reads like a missing backup and was a
# missing binary. A drill whose normal path cannot run is not a drill.
run_json="$(${KUBECTL} -n "${NAMESPACE}" run "rehearsal-reader-$$" --rm -i --restart=Never \
  --image="${CURL_IMAGE}" --quiet -- \
  curl -s -H "${AUTH_HEADER}" \
  http://noryx-backend:8080/api/v1/admin/backups/runs 2>/dev/null || true)"
#
# grep -o, not sed: the runs arrive newest first on a single line, and a greedy
# ".*" anchors on the LAST objectKey of that line rather than the first. The
# drill was therefore rehearsing the oldest backup on record - on this platform
# an empty manual manifest from three months earlier - and reporting the empty
# tables that produced as a restore failure. It was reading the wrong object,
# faithfully.
object_key="${OBJECT_KEY:-$(printf '%s' "${run_json}" | grep -o '"objectKey":"[^"]*"' | head -n 1 | cut -d'"' -f4)}"
if [ -z "${object_key}" ]; then
  # Two different failures, said apart. A refusal is a credential problem and
  # an empty list is a backup problem, and reporting both as the second sent
  # the reader to look at backups that were never in question.
  case "${run_json}" in
    *'"error"'*)
      echo "the platform refused the backup listing: ${run_json}" >&2
      ;;
    *)
      echo "no backup object to restore; set OBJECT_KEY" >&2
      ;;
  esac
  exit 1
fi
say "restoring ${object_key}"

cleanup
${KUBECTL} -n "${NAMESPACE}" exec deployment/postgres -- psql -U noryx -d postgres -c "CREATE DATABASE ${DATABASE}" >/dev/null

# Postgres only admits the platform's own components. The rehearsal is not one
# of them, and its refusal is the network policy working - so it is allowed in
# by a policy that exists for the length of this script and names one label.
cat <<EOF | ${KUBECTL} apply -f - >/dev/null
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: postgres-allow-restore-rehearsal
  namespace: ${NAMESPACE}
spec:
  podSelector:
    matchLabels: { app: postgres }
  policyTypes: [Ingress]
  ingress:
    - from:
        - podSelector:
            matchLabels: { noryx.io/rehearsal: "true" }
      ports: [{ protocol: TCP, port: 5432 }]
EOF

cat <<EOF | ${KUBECTL} apply -f - >/dev/null
apiVersion: apps/v1
kind: Deployment
metadata:
  name: noryx-restore-rehearsal
  namespace: ${NAMESPACE}
  labels: { app: noryx-restore-rehearsal }
spec:
  replicas: 1
  selector: { matchLabels: { app: noryx-restore-rehearsal } }
  template:
    metadata:
      labels: { app: noryx-restore-rehearsal, noryx.io/rehearsal: "true" }
    spec:
      serviceAccountName: noryx-backend
      imagePullSecrets: [{ name: harbor-regcred }]
      containers:
        - name: noryx-backend
          image: ${IMAGE}
          env:
            - { name: NORYX_LISTEN_ADDR, value: ":8080" }
            - { name: NORYX_AUTH_MODE, value: "header" }
            - { name: NORYX_EDITION, value: "enterprise" }
            - { name: NORYX_ENABLE_K8S_RUNTIME, value: "true" }
            # The namespace the backup target secret lives in. Without it the
            # rehearsal looks for it somewhere else and reports "no backup
            # target configured", which reads like a missing backup.
            - { name: NORYX_KUBE_NAMESPACE, value: "${NAMESPACE}" }
            - { name: NORYX_DATABASE_HOST, value: "postgres" }
            - { name: NORYX_DATABASE_NAME, value: "${DATABASE}" }
            - { name: NORYX_DATABASE_USER, value: "noryx" }
            - { name: NORYX_DATABASE_SSLMODE, value: "require" }
            - { name: NORYX_BOOTSTRAP_ADMIN_USER, value: "admin" }
            - name: NORYX_DATABASE_PASSWORD
              valueFrom: { secretKeyRef: { name: noryx-secrets, key: POSTGRES_PASSWORD } }
            - name: NORYX_SECRETS_MASTER_KEY
              valueFrom: { secretKeyRef: { name: noryx-secrets, key: NORYX_SECRETS_MASTER_KEY } }
            # The throwaway platform gets a secret generated for this run and
            # discarded with it. Handing it the live platform's shared secret
            # made the rehearsal a reason that secret could never be retired -
            # a drill holding production's credential is a drill that widens
            # the thing it exists to prove.
            - { name: NORYX_SERVICE_TOKEN, value: "${REHEARSAL_SECRET}" }
          resources:
            requests: { cpu: "100m", memory: "128Mi" }
            limits: { cpu: "1", memory: "1Gi" }
---
apiVersion: v1
kind: Service
metadata: { name: noryx-restore-rehearsal, namespace: ${NAMESPACE} }
spec:
  selector: { app: noryx-restore-rehearsal }
  ports: [{ port: 8080, targetPort: 8080 }]
EOF
${KUBECTL} -n "${NAMESPACE}" rollout status deployment/noryx-restore-rehearsal --timeout=180s >/dev/null

# The same generated secret the copy was started with. It exists for the length
# of this drill and nowhere else.
#
# The answer is kept and shown. Discarding it meant a refused or failed restore
# left the drill reporting empty tables with no reason beside them, and an
# operator reading "restored 0" cannot tell a broken backup from a broken
# credential.
# Retried until the copy answers.
#
# `rollout status` returns when the container is running, and this deployment
# has no readiness probe, so running is not listening: the backend waits up to
# a minute for Postgres before it opens its port. A single request fired at that
# moment came back HTTP 000 - a connection refused, reported as a failed
# restore, with nine CHECK lines under it blaming the backup.
#
# curl's own --retry does not cover a refused connection, so the loop is here.
restore_result=""
for attempt in $(seq 1 20); do
  restore_result="$(${KUBECTL} -n "${NAMESPACE}" run "rehearsal-client-$$-${attempt}" --rm -i --restart=Never --image="${CURL_IMAGE}" --quiet -- \
    curl -s -w '\nHTTP %{http_code}' -X POST -H "X-Noryx-Service-Token: ${REHEARSAL_SECRET}" -H "X-Noryx-User: admin" -H "Content-Type: application/json" \
    -d "{\"mode\":\"missing-only\",\"objectKey\":\"${object_key}\"}" \
    "http://noryx-restore-rehearsal:8080/api/v1/admin/backups/runs/external/restore" 2>&1 || true)"
  case "${restore_result}" in
    *"HTTP 000"*) sleep 6 ;;
    *) break ;;
  esac
done
restore_status="$(printf '%s' "${restore_result}" | grep -o 'HTTP [0-9]*' | tail -n 1)"
echo
echo "  the restore answered ${restore_status:-nothing}"
# The body only when it is not a plain success. On a good run it is several
# thousand characters of collection listings, which buries the table below it;
# on a bad one it is the only thing that says why.
case "${restore_status}" in
  "HTTP 200") ;;
  *) printf '%s\n' "${restore_result}" | sed 's/^/    /' ;;
esac

echo
echo "  what the restored platform holds, against the live one:"
# The objects, then the graph between them.
#
# The graph is the half that cannot be reconstructed from memory: an object can
# be recreated from its definition, but who was allowed to see it, and which
# project it belonged to, lives in people's heads - and a recovery is exactly
# when those people are busy. It went unbacked-up for a long time precisely
# because this table never looked at it.
#
# Counted excluding orphans. A grant whose project no longer exists is a row
# nobody can see and nobody can revoke; the backup does not carry it and the
# restore refuses to create it, both deliberately. Comparing raw counts made
# the drill report a failure every night for a row that should not be there -
# and a drill that cries wolf is a drill people stop reading.
count_live_and_restored() {
  table="$1"
  where="$2"
  for database in "${DATABASE}" noryx; do
    ${KUBECTL} -n "${NAMESPACE}" exec deployment/postgres -- \
      psql -U noryx -d "${database}" -tAc \
      "select count(*) from ${table} ${where}" 2>/dev/null | tr -d ' \r'
  done
}

failures=0
for entry in \
  "projects|" \
  "datasets|" \
  "ontologies|" \
  "repositories|" \
  "apps|" \
  "project_datasets|where project_id in (select id from projects)" \
  "project_repositories|where project_id in (select id from projects)" \
  "dataset_access|where dataset_id in (select id from datasets)" \
  "access_organization_roles|where project_id in (select id from projects)"
do
  table="${entry%%|*}"
  where="${entry#*|}"
  read -r restored live <<EOF
$(count_live_and_restored "${table}" "${where}" | tr '\n' ' ')
EOF
  status="ok  "
  # Live can legitimately have grown since the backup was taken; fewer is what
  # a failed restore looks like.
  if [ "${restored:-0}" -lt "${live:-0}" ]; then
    status="CHECK"
    failures=$((failures + 1))
  fi
  printf '  %s  %-26s restored %-5s live %s\n' "${status}" "${table}" "${restored:-?}" "${live:-?}"
done
echo
echo "  the rehearsal is removed on exit; the live platform was only read from"

# The exit code has to carry the verdict.
#
# It did not: a run where the restore never happened printed nine CHECK lines
# and exited 0. On a schedule that is a drill reporting success while proving
# nothing - the same silent success this whole document exists to prevent, one
# level up. Whoever reads the output sees the problem; whoever reads only the
# exit code is the one this matters for.
if [ "${failures}" -gt 0 ]; then
  echo
  echo "  ${failures} table(s) came back short: the restore did not reproduce the platform" >&2
  exit 1
fi
