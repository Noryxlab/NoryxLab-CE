#!/usr/bin/env bash
#
# A logical dump of the platform database, beside the volume snapshots.
#
#   ./scripts/ops/backup-database.sh [namespace]
#
# Longhorn already copies the volume, so this looks redundant and is not. A
# volume snapshot of a running Postgres is crash-consistent: it restores like a
# server that lost power, and recovery depends on the write-ahead log replaying
# cleanly. It almost always does - and "almost" is carrying weight in a
# sentence about the only copy of the platform's database.
#
# A dump has no such ambiguity. It is a transactionally consistent view taken
# while the server cooperates, it is small enough to keep many of, and it can
# be read into a different Postgres version, which a block-level snapshot
# cannot. The two are not alternatives: the snapshot covers what the dump does
# not (the exact on-disk state, quickly), and the dump covers what the snapshot
# cannot promise.
#
# This database holds Keycloak's tables as well as the platform's, so the dump
# contains password hashes. It is encrypted before it leaves the cluster, with
# the same key as the identity export - it protects the same class of thing,
# and a sixth secret to keep outside the cluster is a sixth secret to lose.
set -euo pipefail

NAMESPACE="${1:-${NAMESPACE:-noryx}}"
KUBECTL="${KUBECTL:-kubectl}"
DATABASE="${DATABASE:-noryx}"
IMAGE="${IDENTITY_BACKUP_IMAGE:-harbor.lan/noryx-ce/noryx-identity-backup:0.1.0}"
KEY_SECRET="${KEY_SECRET:-noryx-identity-backup-key}"
STAMP="$(date -u +%Y/%m/%d/%H%M%SZ)"

if ! ${KUBECTL} -n "${NAMESPACE}" get secret "${KEY_SECRET}" >/dev/null 2>&1; then
  echo "no ${KEY_SECRET} secret: this dump would be a password database in the clear" >&2
  exit 2
fi

POSTGRES_IMAGE="${POSTGRES_IMAGE:-$(${KUBECTL} -n "${NAMESPACE}" get deployment/postgres -o jsonpath='{.spec.template.spec.containers[0].image}')}"

target_value() {
  ${KUBECTL} -n "${NAMESPACE}" get secret noryx-backup-target -o jsonpath="{.data.$1}" | base64 -d
}
ENDPOINT="$(target_value endpoint)"
BUCKET="$(target_value bucket)"
ACCESS_KEY="$(target_value accessKey)"
SECRET_KEY="$(target_value secretKey)"
OBJECT="database/${STAMP}/${DATABASE}.sql.gz.enc"
# mc takes its target as a URL, so the credentials have to be percent-encoded:
# an S3 secret containing a slash or a plus otherwise truncates the host.
ENDPOINT_SCHEME="${ENDPOINT%%://*}"
ENDPOINT_HOST="${ENDPOINT#*://}"
SECRET_KEY_ESCAPED="$(printf '%s' "${SECRET_KEY}" | sed -e 's|/|%2F|g' -e 's|+|%2B|g' -e 's|=|%3D|g')"

job="noryx-database-backup-$(date -u +%s)"
echo "  dumping ${DATABASE} to ${BUCKET}/${OBJECT}"

# The dump runs as an init container so the upload cannot start before it has
# finished - and so a failed dump fails the job rather than shipping an empty
# file that looks like a backup.
cat <<EOF | ${KUBECTL} apply -f - >/dev/null
apiVersion: batch/v1
kind: Job
metadata:
  name: ${job}
  namespace: ${NAMESPACE}
  labels: { app.kubernetes.io/name: noryx-database-backup }
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 600
  template:
    metadata:
      labels:
        app.kubernetes.io/name: noryx-database-backup
        # Named in the policy that guards Postgres. Without it the pod is
        # refused at the network layer and pg_dump reports "connection
        # refused", which reads like a database that is down.
        noryx.io/database-backup: "true"
    spec:
      restartPolicy: Never
      imagePullSecrets: [{ name: harbor-regcred }]
      volumes:
        - { name: dump, emptyDir: {} }
      initContainers:
        - name: dump
          image: ${POSTGRES_IMAGE}
          command: ["/bin/sh", "-c"]
          args:
            - |
              set -e
              # pipefail, and it is the whole point.
              #
              # Without it, pg_dump failing while gzip succeeds gives the
              # pipeline gzip's exit status - zero - so the step passed, a
              # twenty-byte empty archive was encrypted, and forty-eight bytes
              # were shipped as the night's backup. That happened on the first
              # run of this script, under a comment promising it could not.
              set -o pipefail
              # Compressed on the way out: a dump is text, and the difference
              # between shipping 60 MB and 600 MB every night is the
              # difference between a habit and a decision.
              pg_dump -U noryx -h postgres -d "\$DATABASE" | gzip -6 > /dump/db.sql.gz
              taille=\$(wc -c < /dump/db.sql.gz)
              # A floor, because an exit status is not the only way to produce
              # nothing. Anything this small is an empty archive whatever the
              # tools claim, and shipping it would put a file in the bucket
              # that looks like a backup for as long as nobody opens it.
              if [ "\$taille" -lt 4096 ]; then
                echo "dump de \$taille octets : trop petit pour etre une base" >&2
                exit 1
              fi
              echo "dumped \$taille bytes compressed"
          env:
            - { name: DATABASE, value: "${DATABASE}" }
            - name: PGPASSWORD
              valueFrom: { secretKeyRef: { name: noryx-secrets, key: POSTGRES_PASSWORD } }
          volumeMounts: [{ name: dump, mountPath: /dump }]
      containers:
        - name: ship
          image: ${IMAGE}
          command:
            - /bin/sh
            - -c
            - |
              set -e
              openssl enc -aes-256-cbc -pbkdf2 -iter 200000 -salt \\
                -pass env:BACKUP_KEY -in /dump/db.sql.gz -out /tmp/db.sql.gz.enc
              # MC_HOST_target rather than 'mc alias set': the container runs
              # as a non-root user with no writable home, and an alias would
              # need one. It also keeps the credentials out of the argument
              # list.
              mc --config-dir /tmp/.mc cp /tmp/db.sql.gz.enc "target/\$BUCKET/\$OBJECT"
              echo "shipped \$OBJECT (\$(wc -c < /tmp/db.sql.gz.enc) bytes encrypted)"
          env:
            - { name: HOME, value: "/tmp" }
            - { name: BUCKET, value: "${BUCKET}" }
            - { name: OBJECT, value: "${OBJECT}" }
            - name: MC_HOST_target
              value: "${ENDPOINT_SCHEME}://${ACCESS_KEY}:${SECRET_KEY_ESCAPED}@${ENDPOINT_HOST}"
            - name: BACKUP_KEY
              valueFrom: { secretKeyRef: { name: ${KEY_SECRET}, key: key } }
          volumeMounts: [{ name: dump, mountPath: /dump }]
EOF

${KUBECTL} -n "${NAMESPACE}" wait --for=condition=complete "job/${job}" --timeout=900s >/dev/null 2>&1 || true
${KUBECTL} -n "${NAMESPACE}" logs "job/${job}" --all-containers 2>/dev/null | sed 's/^/  /'
if ! ${KUBECTL} -n "${NAMESPACE}" get "job/${job}" -o jsonpath='{.status.succeeded}' 2>/dev/null | grep -q 1; then
  echo "  the dump did not complete; the object above may be absent or partial" >&2
  exit 1
fi
echo "  decrypt with: openssl enc -d -aes-256-cbc -pbkdf2 -iter 200000 -pass env:BACKUP_KEY | gunzip"
