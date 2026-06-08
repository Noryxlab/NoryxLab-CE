#!/usr/bin/env sh
set -eu

NAMESPACE="${NAMESPACE:-kube-system}"
RELEASE="${RELEASE:-csi-s3}"
CHART_VERSION="${CHART_VERSION:-0.43.7}"

helm repo add yandex-s3 https://yandex-cloud.github.io/k8s-csi-s3/charts >/dev/null 2>&1 || true
helm repo update yandex-s3
helm upgrade --install "${RELEASE}" yandex-s3/csi-s3 \
  --version "${CHART_VERSION}" \
  --namespace "${NAMESPACE}" \
  --wait \
  --timeout 5m

kubectl get csidriver ru.yandex.s3.csi
