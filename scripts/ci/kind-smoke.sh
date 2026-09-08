#!/usr/bin/env bash
# Deploy the portable CE composition to the ephemeral Kind cluster created by CI.
set -euo pipefail

CLUSTER_NAME=${KIND_CLUSTER_NAME:-noryx-ci}
NAMESPACE=noryx

cleanup() {
  if [[ -n "${port_forward_pid:-}" ]]; then
    kill "$port_forward_pid" 2>/dev/null || true
  fi
}
trap cleanup EXIT

kubectl kustomize deploy/k8s/ci >/dev/null

docker build -t noryx-backend:ci backend
docker build -t noryx-frontend:ci frontend
kind load docker-image --name "$CLUSTER_NAME" noryx-backend:ci noryx-frontend:ci

kubectl apply --server-side -k deploy/k8s/ci

# The stateful services must be ready before Keycloak and the API can start.
for deployment in postgres minio keycloak noryx-frontend noryx-backend; do
  kubectl -n "$NAMESPACE" rollout status "deployment/${deployment}" --timeout=300s
done

kubectl -n "$NAMESPACE" get pods
kubectl -n "$NAMESPACE" port-forward service/noryx-backend 18080:8080 >/tmp/noryx-kind-port-forward.log 2>&1 &
port_forward_pid=$!
for _ in $(seq 1 20); do
  if curl -fsS http://127.0.0.1:18080/healthz; then
    echo
    echo "Kind deployment smoke: passed"
    exit 0
  fi
  sleep 1
done

cat /tmp/noryx-kind-port-forward.log >&2 || true
exit 1
