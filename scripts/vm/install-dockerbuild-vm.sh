#!/usr/bin/env bash
set -euo pipefail

# Prepares a dedicated dockerbuild VM for Noryx image builds.
# Usage:
#   HARBOR_HOSTNAME=harbor.lan HARBOR_IP=192.168.1.106 ./install-dockerbuild-vm.sh

HARBOR_HOSTNAME="${HARBOR_HOSTNAME:-harbor.lan}"
HARBOR_IP="${HARBOR_IP:-}"

if [ "${EUID}" -ne 0 ]; then
  exec sudo --preserve-env=HARBOR_HOSTNAME,HARBOR_IP bash "$0" "$@"
fi

apt-get update -y
apt-get install -y ca-certificates curl docker.io docker-buildx
systemctl enable --now docker

# Optional hostname mapping for Harbor if local DNS is not configured.
if [ -n "$HARBOR_IP" ]; then
  if ! grep -q "[[:space:]]${HARBOR_HOSTNAME}$" /etc/hosts; then
    echo "${HARBOR_IP} ${HARBOR_HOSTNAME}" >> /etc/hosts
  fi
fi

echo "dockerbuild VM ready."
echo "Next:"
echo "  docker login ${HARBOR_HOSTNAME}"
echo "  docker build -t ${HARBOR_HOSTNAME}/noryx-ce/noryx-backend:<tag> backend/"
echo "  docker push ${HARBOR_HOSTNAME}/noryx-ce/noryx-backend:<tag>"
echo "  docker build -t ${HARBOR_HOSTNAME}/noryx-ce/noryx-frontend:<tag> frontend/"
echo "  docker push ${HARBOR_HOSTNAME}/noryx-ce/noryx-frontend:<tag>"
