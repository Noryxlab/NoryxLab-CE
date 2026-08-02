#!/usr/bin/env bash
set -euo pipefail

# Installs Harbor on a dedicated VM.
# Usage:
#   HARBOR_HOSTNAME=harbor.example.local HARBOR_ADMIN_PASSWORD='***' ./install-harbor-vm.sh

HARBOR_VERSION="${HARBOR_VERSION:-2.10.2}"
HARBOR_HOSTNAME="${HARBOR_HOSTNAME:-harbor.example.local}"
HARBOR_ADMIN_PASSWORD="${HARBOR_ADMIN_PASSWORD:-}"
HARBOR_INSTALL_DIR="${HARBOR_INSTALL_DIR:-/opt/harbor}"

if [ -z "$HARBOR_ADMIN_PASSWORD" ]; then
  echo "HARBOR_ADMIN_PASSWORD is required." >&2
  exit 1
fi

if [ "${EUID}" -ne 0 ]; then
  exec sudo --preserve-env=HARBOR_VERSION,HARBOR_HOSTNAME,HARBOR_ADMIN_PASSWORD,HARBOR_INSTALL_DIR bash "$0" "$@"
fi

apt-get update -y
COMPOSE_PACKAGE="docker-compose-plugin"
if ! apt-cache show docker-compose-plugin >/dev/null 2>&1; then
  COMPOSE_PACKAGE="docker-compose"
fi
apt-get install -y ca-certificates curl openssl docker.io "$COMPOSE_PACKAGE" jq rsync python3
systemctl enable --now docker

mkdir -p "$HARBOR_INSTALL_DIR"
cd "$HARBOR_INSTALL_DIR"

TARBALL="harbor-offline-installer-v${HARBOR_VERSION}.tgz"
URL="https://github.com/goharbor/harbor/releases/download/v${HARBOR_VERSION}/${TARBALL}"
[ -f "$TARBALL" ] || curl -fsSL -o "$TARBALL" "$URL"

tar -xzf "$TARBALL"
cd harbor
cp -n harbor.yml.tmpl harbor.yml

mkdir -p certs
if [ ! -f certs/harbor.crt ] || [ ! -f certs/harbor.key ]; then
  openssl req -x509 -nodes -days 3650 -newkey rsa:4096 \
    -subj "/CN=${HARBOR_HOSTNAME}" \
    -addext "subjectAltName=DNS:${HARBOR_HOSTNAME}" \
    -keyout certs/harbor.key \
    -out certs/harbor.crt
fi

sed -i "s/^hostname:.*$/hostname: ${HARBOR_HOSTNAME}/" harbor.yml
sed -i "s/^harbor_admin_password:.*$/harbor_admin_password: ${HARBOR_ADMIN_PASSWORD}/" harbor.yml
sed -i 's/^# *https:/https:/' harbor.yml
sed -i 's/^# *  port: 443/  port: 443/' harbor.yml
sed -i "s|^  certificate:.*$|  certificate: ${HARBOR_INSTALL_DIR}/harbor/certs/harbor.crt|" harbor.yml
sed -i "s|^  private_key:.*$|  private_key: ${HARBOR_INSTALL_DIR}/harbor/certs/harbor.key|" harbor.yml

./install.sh
echo "Harbor installed: https://${HARBOR_HOSTNAME}"
