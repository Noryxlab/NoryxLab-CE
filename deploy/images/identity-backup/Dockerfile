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
FROM minio/mc:RELEASE.2025-04-16T18-13-26Z AS mc

FROM alpine:3.20
RUN apk add --no-cache openssl ca-certificates
COPY --from=mc /usr/bin/mc /usr/bin/mc
# An identity export holds password hashes. Running as a non-root user does not
# make that safe, but it removes the reason to wonder.
USER 10001
ENTRYPOINT ["/bin/sh"]
