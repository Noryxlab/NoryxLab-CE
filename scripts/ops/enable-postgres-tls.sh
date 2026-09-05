#!/usr/bin/env bash
#
# Gives Postgres a certificate, so the platform stops talking to its database
# in the clear.
#
#   ./scripts/ops/enable-postgres-tls.sh [namespace]
#
# The certificate is self-signed and that is the honest choice: this connection
# never leaves the cluster, the server is reached by its service name, and a
# public authority has nothing to say about `postgres.noryx.svc.cluster.local`.
# What it buys is encryption on the wire - `sslmode=require`. It does not buy
# authentication of the server, which would need the certificate pinned on the
# client too; the network policies restricting who can reach port 5432 are what
# stands in for that today.
#
# Idempotent: an existing secret is left alone, because replacing it means
# restarting the database.
set -euo pipefail

NAMESPACE="${1:-${NAMESPACE:-noryx}}"
KUBECTL="${KUBECTL:-kubectl}"
SERVICE="${SERVICE:-postgres}"
DAYS="${DAYS:-3650}"

if ${KUBECTL} -n "${NAMESPACE}" get secret postgres-tls >/dev/null 2>&1; then
  echo "  postgres-tls already exists in ${NAMESPACE}; leaving it alone"
  exit 0
fi

work="$(mktemp -d)"
trap 'rm -rf "${work}"' EXIT

# Every name the server is reached by. A name missing here is a client that
# cannot verify, the day somebody moves from sslmode=require to verify-full.
openssl req -x509 -nodes -newkey rsa:2048 -days "${DAYS}" \
  -keyout "${work}/tls.key" -out "${work}/tls.crt" \
  -subj "/CN=${SERVICE}" \
  -addext "subjectAltName=DNS:${SERVICE},DNS:${SERVICE}.${NAMESPACE},DNS:${SERVICE}.${NAMESPACE}.svc,DNS:${SERVICE}.${NAMESPACE}.svc.cluster.local" \
  >/dev/null 2>&1

${KUBECTL} -n "${NAMESPACE}" create secret tls postgres-tls \
  --cert="${work}/tls.crt" --key="${work}/tls.key" >/dev/null

echo "  postgres-tls created in ${NAMESPACE}, valid ${DAYS} days"
echo "  restart Postgres for it to serve TLS, then set NORYX_DATABASE_SSLMODE=require:"
echo "    ${KUBECTL} -n ${NAMESPACE} rollout restart deployment/postgres"
