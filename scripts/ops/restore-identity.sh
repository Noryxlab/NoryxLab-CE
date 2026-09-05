#!/usr/bin/env bash
#
# Puts the accounts back: users with their password hashes, the organizations,
# and the memberships.
#
#   NORYX_IDENTITY_BACKUP_KEY=... ./scripts/ops/restore-identity.sh <object-key> [namespace]
#
# Rehearsal, which is what you should run first and on a schedule:
#
#   REHEARSE=1 NORYX_IDENTITY_BACKUP_KEY=... ./scripts/ops/restore-identity.sh <object-key>
#
# A rehearsal imports into a throwaway database and counts what arrived. A real
# restore imports into the platform's own database and *replaces* the realm,
# which is right when recovering and catastrophic when not - so it refuses
# unless the realm is absent or ALLOW_REALM_OVERWRITE=1 says otherwise.
set -euo pipefail

OBJECT="${1:-}"
NAMESPACE="${2:-${NAMESPACE:-noryx}}"
KUBECTL="${KUBECTL:-kubectl}"
REALM="${REALM:-noryx}"
IMAGE="${IDENTITY_BACKUP_IMAGE:-harbor.lan/noryx-ce/noryx-identity-backup:0.1.0}"
REHEARSAL_DATABASE="${REHEARSAL_DATABASE:-noryx_identity_rehearsal}"

if [ -z "${OBJECT}" ]; then
  echo "usage: $0 <object-key> [namespace]" >&2
  exit 2
fi
if [ -z "${NORYX_IDENTITY_BACKUP_KEY:-}" ]; then
  echo "NORYX_IDENTITY_BACKUP_KEY is required: the object is encrypted and the key is not in the backup" >&2
  exit 2
fi

target_value() {
  ${KUBECTL} -n "${NAMESPACE}" get secret noryx-backup-target -o jsonpath="{.data.$1}" | base64 -d
}
ENDPOINT="$(target_value endpoint)"
BUCKET="$(target_value bucket)"
ACCESS_KEY="$(target_value accessKey)"
SECRET_KEY="$(target_value secretKey)"
ENDPOINT_SCHEME="${ENDPOINT%%://*}"
ENDPOINT_HOST="${ENDPOINT#*://}"
SECRET_KEY_ESCAPED="$(printf '%s' "${SECRET_KEY}" | sed -e 's|/|%2F|g' -e 's|+|%2B|g' -e 's|=|%3D|g')"
KEYCLOAK_IMAGE="$(${KUBECTL} -n "${NAMESPACE}" get deployment/keycloak -o jsonpath='{.spec.template.spec.containers[0].image}')"

DATABASE="noryx"
if [ "${REHEARSE:-0}" = "1" ]; then
  DATABASE="${REHEARSAL_DATABASE}"
  ${KUBECTL} -n "${NAMESPACE}" exec deployment/postgres -- psql -U noryx -d postgres -c "DROP DATABASE IF EXISTS ${DATABASE}" >/dev/null
  ${KUBECTL} -n "${NAMESPACE}" exec deployment/postgres -- psql -U noryx -d postgres -c "CREATE DATABASE ${DATABASE}" >/dev/null
  echo "  rehearsing into ${DATABASE}, the live realm is untouched"
elif [ "${ALLOW_REALM_OVERWRITE:-0}" != "1" ]; then
  # Importing replaces the realm. On a platform that still has its accounts,
  # that is how a restore turns into an incident.
  existing="$(${KUBECTL} -n "${NAMESPACE}" exec deployment/postgres -- psql -U noryx -d noryx -tAc "select count(*) from realm where name='${REALM}'" 2>/dev/null | tr -d ' \r' || echo 0)"
  if [ "${existing:-0}" != "0" ]; then
    echo "realm ${REALM} already exists: importing would replace it. Rehearse first (REHEARSE=1), or set ALLOW_REALM_OVERWRITE=1 deliberately." >&2
    exit 1
  fi
fi

job="noryx-identity-restore-$(date -u +%s)"
cat <<EOF | ${KUBECTL} apply -f - >/dev/null
apiVersion: batch/v1
kind: Job
metadata:
  name: ${job}
  namespace: ${NAMESPACE}
  labels: { app.kubernetes.io/name: noryx-identity-restore }
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 600
  template:
    metadata:
      labels: { app.kubernetes.io/name: noryx-identity-restore, noryx.io/identity-backup: "true" }
    spec:
      restartPolicy: Never
      imagePullSecrets: [{ name: harbor-regcred }]
      volumes:
        - { name: export, emptyDir: {} }
      initContainers:
        - name: fetch
          image: ${IMAGE}
          command:
            - /bin/sh
            - -c
            - |
              set -e
              mc --config-dir /tmp/.mc cp "target/\$BUCKET/\$OBJECT" /tmp/identity.tar.enc
              openssl enc -d -aes-256-cbc -pbkdf2 -iter 200000 -pass env:BACKUP_KEY -in /tmp/identity.tar.enc | tar xf - -C /export
              ls -la /export
          env:
            - { name: HOME, value: "/tmp" }
            - { name: BUCKET, value: "${BUCKET}" }
            - { name: OBJECT, value: "${OBJECT}" }
            - { name: BACKUP_KEY, value: "${NORYX_IDENTITY_BACKUP_KEY}" }
            - name: MC_HOST_target
              value: "${ENDPOINT_SCHEME}://${ACCESS_KEY}:${SECRET_KEY_ESCAPED}@${ENDPOINT_HOST}"
          volumeMounts: [{ name: export, mountPath: /export }]
      containers:
        - name: import
          image: ${KEYCLOAK_IMAGE}
          command: ["/opt/keycloak/bin/kc.sh", "import", "--dir", "/export", "--override", "true"]
          env:
            - { name: KC_DB, value: postgres }
            - { name: KC_DB_URL, value: "jdbc:postgresql://postgres:5432/${DATABASE}?sslmode=prefer" }
            - { name: KC_DB_USERNAME, value: noryx }
            - name: KC_DB_PASSWORD
              valueFrom: { secretKeyRef: { name: noryx-secrets, key: POSTGRES_PASSWORD } }
          volumeMounts: [{ name: export, mountPath: /export, readOnly: true }]
EOF

for _ in $(seq 1 120); do
  succeeded="$(${KUBECTL} -n "${NAMESPACE}" get "job/${job}" -o jsonpath='{.status.succeeded}' 2>/dev/null || true)"
  failed="$(${KUBECTL} -n "${NAMESPACE}" get "job/${job}" -o jsonpath='{.status.failed}' 2>/dev/null || true)"
  [ -n "${succeeded:-}" ] && [ "${succeeded}" != "0" ] && break
  [ -n "${failed:-}" ] && [ "${failed}" != "0" ] && break
  sleep 5
done
if [ -z "${succeeded:-}" ] || [ "${succeeded}" = "0" ]; then
  echo "  the restore job did not complete; its logs:" >&2
  ${KUBECTL} -n "${NAMESPACE}" logs "job/${job}" --all-containers --tail=25 >&2 || true
  exit 1
fi

echo "  what arrived in ${DATABASE}:"
for query in "select 'accounts: '||count(*) from user_entity" \
             "select 'with a password: '||count(*) from credential where type='password'" \
             "select 'organizations: '||count(*) from org" ; do
  ${KUBECTL} -n "${NAMESPACE}" exec deployment/postgres -- psql -U noryx -d "${DATABASE}" -tAc "${query}" 2>/dev/null | sed 's/^/    /'
done
${KUBECTL} -n "${NAMESPACE}" delete "job/${job}" >/dev/null 2>&1 || true

if [ "${REHEARSE:-0}" = "1" ]; then
  ${KUBECTL} -n "${NAMESPACE}" exec deployment/postgres -- psql -U noryx -d postgres -c "DROP DATABASE IF EXISTS ${DATABASE}" >/dev/null
  echo "  rehearsal database removed"
fi
