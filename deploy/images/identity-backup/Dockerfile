# The tools an identity backup needs, and nothing else.
#
# Keycloak's own image carries no openssl, no tar and no S3 client - it is
# deliberately minimal - and mc's image carries no openssl either. Rather than
# stitch a pipeline across three containers to work around that, this image
# holds exactly the two binaries the job needs: mc to reach the off-site bucket
# and openssl to encrypt before it gets there.
#
# It never talks to Keycloak: the export is produced by Keycloak itself into a
# shared volume, and this image only encrypts and ships it.
# Pinned by digest, not by tag.
#
# The tag this used - RELEASE.2025-04-16T18-13-26Z - no longer resolves: MinIO
# retired it, and the recipe for the platform's identity backup became
# unbuildable without anybody noticing, because nobody rebuilds an image that
# already exists. A digest cannot be retagged or withdrawn, which is the whole
# point of pinning; the comment beside it says which release it was, since a
# digest tells a reader nothing.
#
# mc, built 2025-09-07.
FROM minio/mc@sha256:a7fe349ef4bd8521fb8497f55c6042871b2ae640607cf99d9bede5e9bdf11727 AS mc

FROM alpine:3.20
RUN apk add --no-cache openssl ca-certificates
COPY --from=mc /usr/bin/mc /usr/bin/mc
# An identity export holds password hashes. Running as a non-root user does not
# make that safe, but it removes the reason to wonder.
USER 10001
ENTRYPOINT ["/bin/sh"]
