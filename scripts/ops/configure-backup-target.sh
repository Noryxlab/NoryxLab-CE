#!/usr/bin/env bash
#
# Points an installation's backups at an object store.
#
#   ENDPOINT=https://cellar-c2.services.clever-cloud.com \
#   BUCKET=noryx-emse ACCESS_KEY=... SECRET_KEY=... \
#     ./scripts/ops/configure-backup-target.sh [namespace]
#
# Without this secret the platform schedules backup runs that have nowhere to
# go: the scheduler records the intent, no object is ever written, and the
# installation looks backed up because something is on the calendar. EMSE ran
# that way for 34 scheduled runs and zero stored bytes.
#
# The credentials are verified before they are stored - a target that is only
# written down is the same failure in a different place - and the bucket is
# created if the credentials are allowed to create it.
#
# An endpoint inside the cluster is refused unless ALLOW_IN_CLUSTER_TARGET=1:
# a backup that lives in the cluster it protects covers a dropped table, and
# covers nothing at all on the day the cluster is lost.
set -euo pipefail

NAMESPACE="${1:-${NAMESPACE:-noryx}}"
KUBECTL="${KUBECTL:-kubectl}"
IMAGE="${IDENTITY_BACKUP_IMAGE:-harbor.lan/noryx-ce/noryx-identity-backup:0.1.0}"
PREFIX="${PREFIX:-}"
REGION="${REGION:-us-east-1}"

for v in ENDPOINT BUCKET ACCESS_KEY SECRET_KEY; do
  if [ -z "${!v:-}" ]; then echo "missing ${v}" >&2; exit 2; fi
done

case "${ENDPOINT}" in
  *.svc|*.svc.cluster.local|*.svc:*|*.svc.cluster.local:*|*://minio*|*localhost*|*127.0.0.1*)
    if [ "${ALLOW_IN_CLUSTER_TARGET:-0}" != "1" ]; then
      echo "refusing ${ENDPOINT}: this bucket lives in the cluster the backup is meant to survive" >&2
      echo "  set ALLOW_IN_CLUSTER_TARGET=1 to accept it as an interim target" >&2
      exit 2
    fi
    echo "  warning  ${ENDPOINT} is in-cluster: this protects data loss, not cluster loss"
    ;;
esac

ENDPOINT_SCHEME="${ENDPOINT%%://*}"
ENDPOINT_HOST="${ENDPOINT#*://}"
SECRET_KEY_ESCAPED="$(printf '%s' "${SECRET_KEY}" | sed -e 's|/|%2F|g' -e 's|+|%2B|g' -e 's|=|%3D|g')"

# The check runs in the cluster, from the network the backups will run from:
# credentials that work from an operator's laptop prove nothing about what the
# backup job can reach.
job="noryx-backup-target-check-$(date -u +%s)"
echo "  checking ${BUCKET} on ${ENDPOINT_HOST}"
cat <<EOF | ${KUBECTL} apply -f - >/dev/null
apiVersion: batch/v1
kind: Job
metadata:
  name: ${job}
  namespace: ${NAMESPACE}
  labels: { app.kubernetes.io/name: noryx-backup-target-check }
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 300
  template:
    spec:
      restartPolicy: Never
      containers:
        - name: check
          image: ${IMAGE}
          command: ["/bin/sh","-c"]
          args:
            - |
              set -e
              mc alias set target "${ENDPOINT_SCHEME}://${ENDPOINT_HOST}" "${ACCESS_KEY}" "${SECRET_KEY}" >/dev/null
              mc ls "target/${BUCKET}" >/dev/null 2>&1 || mc mb "target/${BUCKET}"
              echo probe > /tmp/probe
              mc cp /tmp/probe "target/${BUCKET}/${PREFIX}.noryx-target-check" >/dev/null
              mc rm "target/${BUCKET}/${PREFIX}.noryx-target-check" >/dev/null
              echo "the target accepts a write and a delete"
EOF
if ! ${KUBECTL} -n "${NAMESPACE}" wait --for=condition=complete --timeout=180s "job/${job}" >/dev/null 2>&1; then
  echo "  the target refused the check - nothing was stored:" >&2
  ${KUBECTL} -n "${NAMESPACE}" logs "job/${job}" 2>&1 | sed 's/^/    /' >&2
  ${KUBECTL} -n "${NAMESPACE}" delete job "${job}" >/dev/null 2>&1 || true
  exit 1
fi
${KUBECTL} -n "${NAMESPACE}" logs "job/${job}" 2>/dev/null | sed 's/^/  /'
${KUBECTL} -n "${NAMESPACE}" delete job "${job}" >/dev/null 2>&1 || true

${KUBECTL} -n "${NAMESPACE}" create secret generic noryx-backup-target \
  --from-literal=endpoint="${ENDPOINT}" \
  --from-literal=bucket="${BUCKET}" \
  --from-literal=prefix="${PREFIX}" \
  --from-literal=region="${REGION}" \
  --from-literal=accessKey="${ACCESS_KEY}" \
  --from-literal=secretKey="${SECRET_KEY}" \
  --dry-run=client -o yaml | ${KUBECTL} apply -f - >/dev/null

echo "  backups now go to ${BUCKET}${PREFIX:+/${PREFIX}}"
echo "  the credentials are in the secret noryx-backup-target: back them up with scripts/ops/backup-credentials.sh"
