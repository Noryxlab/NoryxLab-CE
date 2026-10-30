#!/usr/bin/env bash
#
# Builds the MinIO image this platform runs, from MinIO's own AGPL-3.0 source.
#
#   scripts/ops/build-minio-image.sh <registry> [release-tag] [revision]
#   PUSH=0 scripts/ops/build-minio-image.sh harbor.example.local
#
# Why this exists, and why it is a build rather than a mirror.
#
# MinIO stopped distributing community container images: on 2026-10-30
# `docker pull minio/minio` and `quay.io/minio/minio` both answer "access
# denied" for the whole repository, not for one release. And its last
# open-source release is RELEASE.2025-10-15 - the six before it run from May to
# October 2025, and nothing has been published since. The community edition
# moved behind their commercial product.
#
# So the mirror documented in docs/IMAGE_MIRRORS.md cannot be refreshed: there
# is nothing to mirror. What remains available is the source, under AGPL-3.0,
# and that is what this builds.
#
# It is not a reproduction of the upstream image, and the differences are
# deliberate:
#
#   - a current Go toolchain. The published image was built with Go 1.23.6 and
#     carried the standard library's accumulated advisories; built with 1.26,
#     those are gone. That single change removed the majority of the findings.
#   - no `mc`. The upstream image ships the client beside the server; our pod
#     runs `minio server` and nothing in the cluster execs `mc` inside it, so
#     it is a second binary's worth of exposure for no use.
#   - two dependencies raised past their advisories - the RabbitMQ client, for
#     event notifications this platform does not configure, and gRPC. Upstream
#     will not be bumping them.
#   - a distroless base rather than a distribution image, which removes the
#     package-manager surface the server never uses.
#
# Measured with trivy before and after, critical and high: 7 and 128 for the
# published image, 0 and 32 for this one.
#
# Root, like the image it replaces. The volume in service was written by root
# and its subdirectories belong to root, so a non-root user would need a chown
# of the whole dataset volume before MinIO could write. That is a migration of
# its own and not a side effect of a version bump.
set -euo pipefail

REGISTRY=${1:-${HARBOR_REGISTRY_HOST:-}}
TAG=${2:-RELEASE.2025-10-15T17-29-55Z}
REVISION=${3:-noryx3}
PUSH=${PUSH:-1}

if [ -z "$REGISTRY" ]; then
  echo "usage: $0 <registry> [release-tag] [revision]" >&2
  echo "  PUSH=0 to build without pushing" >&2
  exit 2
fi

IMAGE="$REGISTRY/noryx-ce/minio:$TAG-$REVISION"
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT
cd "$WORK"

echo "== source $TAG"
curl -fsSL "https://github.com/minio/minio/archive/refs/tags/$TAG.tar.gz" -o minio.tar.gz
# Printed, not checked against a constant: the archive GitHub serves for a tag
# is stable, and a digest recorded here would be one more number to keep true.
# Compare it between the hosts that build the same release.
echo "   archive sha256 : $(sha256sum minio.tar.gz | cut -d' ' -f1)"
tar xzf minio.tar.gz
cd minio-*

cat > Dockerfile.noryx <<'DOCKER'
FROM golang:1.26.10 AS builder
WORKDIR /src
COPY . .
ARG MINIO_VERSION=DEVELOPMENT
# The two dependencies that carry every remaining critical once the toolchain
# is current. If either refuses to compile, this build fails rather than
# shipping a binary nobody checked.
RUN go get google.golang.org/grpc@v1.79.3 github.com/rabbitmq/amqp091-go@v1.13.0 \
 && go mod tidy
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath \
      -ldflags "-s -w -X github.com/minio/minio/cmd.Version=${MINIO_VERSION} -X github.com/minio/minio/cmd.ReleaseTag=${MINIO_VERSION} -X github.com/minio/minio/cmd.CommitID=${MINIO_VERSION}" \
      -o /out/minio .

FROM gcr.io/distroless/static-debian12
LABEL org.opencontainers.image.title="noryx-ce-minio"
LABEL org.opencontainers.image.description="MinIO, built from its AGPL-3.0 source with a current Go toolchain, because upstream no longer distributes community images."
LABEL org.opencontainers.image.source="https://github.com/minio/minio"
LABEL org.opencontainers.image.licenses="AGPL-3.0"
COPY --from=builder /out/minio /usr/bin/minio
ENTRYPOINT ["/usr/bin/minio"]
DOCKER

echo "== build $IMAGE"
docker build --build-arg "MINIO_VERSION=$TAG" -t "$IMAGE" -f Dockerfile.noryx .

echo "== what the binary reports"
docker run --rm --entrypoint /usr/bin/minio "$IMAGE" --version

if [ "$PUSH" = "1" ]; then
  echo "== push"
  docker push -q "$IMAGE"
  echo
  echo "Deploy it, or the cluster keeps the one it has:"
  echo "  kubectl -n noryx set image deploy/minio minio=$IMAGE"
  echo "  (the container is called 'minio'; a guessed name makes set image"
  echo "   succeed silently and change nothing)"
  echo
  echo "Verify with the data, not with the rollout: run the platform"
  echo "validator's daily suite, which writes an object into a dataset and"
  echo "reads it back, and list the objects of a dataset that existed before."
else
  echo "built, not pushed (PUSH=1 to push)"
fi
