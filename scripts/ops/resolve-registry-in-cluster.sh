#!/usr/bin/env bash
#
# Makes the registry resolvable from inside the cluster.
#
#   REGISTRY_HOST=harbor.emse.local REGISTRY_IP=10.210.53.14 \
#     ./scripts/ops/resolve-registry-in-cluster.sh
#
# An installation whose registry is named in /etc/hosts on the nodes - the
# normal arrangement for a private registry with no DNS entry - gives the
# kubelet everything it needs and a pod nothing: cluster DNS never reads that
# file. Images pull, and every in-cluster build fails at the push with "no such
# host", which makes building an environment impossible and says so only in a
# log the platform could not read either.
#
# The entry goes in its own server block, not as a second `hosts` plugin in the
# default one: CoreDNS allows that plugin once per block and crash-loops
# otherwise - taking cluster DNS down with it, which is a worse day than the
# one you started with.
set -euo pipefail

REGISTRY_HOST="${REGISTRY_HOST:-}"
REGISTRY_IP="${REGISTRY_IP:-}"
KUBECTL="${KUBECTL:-kubectl}"
NAMESPACE="${COREDNS_NAMESPACE:-kube-system}"

if [ -z "${REGISTRY_HOST}" ] || [ -z "${REGISTRY_IP}" ]; then
  echo "REGISTRY_HOST and REGISTRY_IP are required" >&2
  exit 2
fi

# k3s reads *.server and *.override from this ConfigMap. A distribution that
# does not mount it will ignore this file, so the check at the end is what
# tells you whether it worked.
if ! ${KUBECTL} -n "${NAMESPACE}" get deployment coredns \
  -o jsonpath='{.spec.template.spec.volumes[*].configMap.name}' 2>/dev/null | grep -q coredns-custom; then
  echo "  coredns does not mount a coredns-custom ConfigMap on this cluster;" >&2
  echo "  add the entry to its Corefile by hand instead" >&2
  exit 1
fi

${KUBECTL} apply -f - <<EOF >/dev/null
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns-custom
  namespace: ${NAMESPACE}
data:
  registry.server: |
    ${REGISTRY_HOST}:53 {
      hosts {
        ${REGISTRY_IP} ${REGISTRY_HOST}
      }
    }
EOF

${KUBECTL} -n "${NAMESPACE}" delete pod -l k8s-app=kube-dns --wait=false >/dev/null 2>&1 || true
echo "  coredns restarting with ${REGISTRY_HOST} -> ${REGISTRY_IP}"

# Wait for DNS to come back before claiming anything: an override that crashes
# CoreDNS looks like success right up to the moment nothing in the cluster can
# resolve anything.
for _ in $(seq 1 30); do
  sleep 2
  ready="$(${KUBECTL} -n "${NAMESPACE}" get pods -l k8s-app=kube-dns \
    -o jsonpath='{range .items[*]}{.status.containerStatuses[0].ready}{"\n"}{end}' 2>/dev/null | grep -c true || true)"
  if [ "${ready}" -ge 1 ]; then
    echo "  cluster DNS is serving again"
    exit 0
  fi
done

echo "  cluster DNS did not come back; remove the override and restart coredns:" >&2
echo "    kubectl -n ${NAMESPACE} delete configmap coredns-custom" >&2
exit 1
