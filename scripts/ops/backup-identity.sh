#!/usr/bin/env bash
#
# Backs up the accounts themselves: users with their password hashes, the
# organizations, and the memberships.
#
#   ./scripts/ops/backup-identity.sh [namespace]
#
# The platform's own backup holds projects, datasets and grants. It does not
# hold a single account, so a restore from it produces a working platform that
# nobody can sign in to. This is the other half.
#
# It runs Keycloak's own exporter, which is the only path that includes
# credentials - the admin API deliberately never returns them. The export is
# therefore a password database in a file, and it is encrypted before it leaves
# the cluster, with a key that is not in the backup.
#
# The key is NORYX_IDENTITY_BACKUP_KEY. Losing it means losing the ability to
# restore accounts; storing it beside the backup means the encryption bought
# nothing. Keep it where the Keycloak admin password is kept.
set -euo pipefail

NAMESPACE="${1:-${NAMESPACE:-noryx}}"
KUBECTL="${KUBECTL:-kubectl}"
REALM="${REALM:-noryx}"
IMAGE="${IDENTITY_BACKUP_IMAGE:-harbor.lan/noryx-ce/noryx-identity-backup:0.1.0}"
KEYCLOAK_IMAGE="${KEYCLOAK_IMAGE:-}"
STAMP="$(date -u +%Y/%m/%d/%H%M%SZ)"

if [ -z "${NORYX_IDENTITY_BACKUP_KEY:-}" ]; then
  echo "NORYX_IDENTITY_BACKUP_KEY is required: it is what keeps a file of password hashes from being readable off-site" >&2
  exit 2
fi
if [ -z "${KEYCLOAK_IMAGE}" ]; then
  KEYCLOAK_IMAGE="$(${KUBECTL} -n "${NAMESPACE}" get deployment/keycloak -o jsonpath='{.spec.template.spec.containers[0].image}')"
fi

target_value() {
  ${KUBECTL} -n "${NAMESPACE}" get secret noryx-backup-target -o jsonpath="{.data.$1}" | base64 -d
}
ENDPOINT="$(target_value endpoint)"
BUCKET="$(target_value bucket)"
ACCESS_KEY="$(target_value accessKey)"
SECRET_KEY="$(target_value secretKey)"
OBJECT="identity/${STAMP}/${REALM}-identity.tar.enc"
# mc takes its target as a URL, so the credentials have to be percent-encoded:
# an S3 secret containing a slash or a plus otherwise truncates the host.
ENDPOINT_SCHEME="${ENDPOINT%%://*}"
ENDPOINT_HOST="${ENDPOINT#*://}"
SECRET_KEY_ESCAPED="$(printf '%s' "${SECRET_KEY}" | sed -e 's|/|%2F|g' -e 's|+|%2B|g' -e 's|=|%3D|g')"

job="noryx-identity-backup-$(date -u +%s)"
echo "  exporting realm ${REALM} to ${BUCKET}/${OBJECT}"

# The export runs as an init container so the upload cannot start before it has
# finished - and so a failed export fails the job rather than shipping an empty
# archive that looks like a backup.
cat <<EOF | ${KUBECTL} apply -f - >/dev/null
apiVersion: batch/v1
kind: Job
metadata:
  name: ${job}
  namespace: ${NAMESPACE}
  labels: { app.kubernetes.io/name: noryx-identity-backup }
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 600
  template:
    metadata:
      labels: { app.kubernetes.io/name: noryx-identity-backup, noryx.io/identity-backup: "true" }
    spec:
      restartPolicy: Never
      imagePullSecrets: [{ name: harbor-regcred }]
      volumes:
        - { name: export, emptyDir: {} }
      initContainers:
        - name: export
          image: ${KEYCLOAK_IMAGE}
          command: ["/opt/keycloak/bin/kc.sh", "export", "--dir", "/export", "--users", "same_file", "--realm", "${REALM}"]
          env:
            - { name: KC_DB, value: postgres }
            - { name: KC_DB_URL, value: "jdbc:postgresql://postgres:5432/noryx?sslmode=prefer" }
            - { name: KC_DB_USERNAME, value: noryx }
            - name: KC_DB_PASSWORD
              valueFrom: { secretKeyRef: { name: noryx-secrets, key: POSTGRES_PASSWORD } }
          volumeMounts: [{ name: export, mountPath: /export }]
      containers:
        - name: ship
          image: ${IMAGE}
          command:
            - /bin/sh
            - -c
            - |
              set -e
              cd /export
              tar cf - . | openssl enc -aes-256-cbc -pbkdf2 -iter 200000 -salt -pass env:BACKUP_KEY -out /tmp/identity.tar.enc
              # MC_HOST_target rather than `mc alias set`: the container runs as
              # a non-root user with no writable home, and an alias would need
              # one. It also keeps the credentials out of the argument list.
              mc --config-dir /tmp/.mc cp /tmp/identity.tar.enc "target/\$BUCKET/\$OBJECT"
              echo "shipped \$OBJECT (\$(wc -c < /tmp/identity.tar.enc) bytes encrypted)"
          env:
            - { name: HOME, value: "/tmp" }
            - { name: BUCKET, value: "${BUCKET}" }
            - { name: OBJECT, value: "${OBJECT}" }
            - name: MC_HOST_target
              value: "${ENDPOINT_SCHEME}://${ACCESS_KEY}:${SECRET_KEY_ESCAPED}@${ENDPOINT_HOST}"
            - { name: BACKUP_KEY, value: "${NORYX_IDENTITY_BACKUP_KEY}" }
          volumeMounts: [{ name: export, mountPath: /export, readOnly: true }]
EOF

# Waiting for either outcome rather than only for success: waiting ten minutes
# for a job that failed in twenty seconds tells nobody anything.
for _ in $(seq 1 120); do
  succeeded="$(${KUBECTL} -n "${NAMESPACE}" get "job/${job}" -o jsonpath='{.status.succeeded}' 2>/dev/null || true)"
  failed="$(${KUBECTL} -n "${NAMESPACE}" get "job/${job}" -o jsonpath='{.status.failed}' 2>/dev/null || true)"
  [ "${succeeded:-0}" != "0" ] && [ -n "${succeeded:-}" ] && break
  [ "${failed:-0}" != "0" ] && [ -n "${failed:-}" ] && break
  sleep 5
done
[ "${succeeded:-0}" != "0" ] && [ -n "${succeeded:-}" ] || {
  echo "  the export job did not complete; its logs:" >&2
  ${KUBECTL} -n "${NAMESPACE}" logs "job/${job}" --all-containers --tail=20 >&2 || true
  exit 1
}
${KUBECTL} -n "${NAMESPACE}" logs "job/${job}" -c ship --tail=3 | sed 's/^/  /'
${KUBECTL} -n "${NAMESPACE}" delete "job/${job}" >/dev/null 2>&1 || true
echo "  keep NORYX_IDENTITY_BACKUP_KEY: without it this object is noise"
