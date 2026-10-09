# Public image mirrors

Noryx CE installations mirror their runtime dependencies into the customer's
own registry before deployment. The catalog is
[`deploy/images/essential-images.txt`](../deploy/images/essential-images.txt).

## MinIO

The MinIO release used by the CE baseline is published at:

```text
ghcr.io/noryxlab/noryx-ce-minio@sha256:5c1e0c9ca1c34b1f2a0f66345f5e2acd2efe98b3aa91089b5cf8ff08b593cda7
```

This is a byte-for-byte registry mirror of the upstream image
`minio/minio:RELEASE.2025-02-18T16-25-55Z`, mirrored on 2026-10-09 after
upstream registries stopped accepting anonymous pulls for that release. It is
published only so a new CE installation can bootstrap its own private Harbor
or equivalent registry.

The mirror contains no Noryx source code, Noryx CE images, Noryx Enterprise
Edition images, configuration, credentials or customer data. MinIO is licensed
under AGPL-3.0; its upstream project is <https://github.com/minio/minio>.

## Maintenance policy

- Pin all consumers to a manifest digest, never a mutable tag.
- Before replacing a mirror, verify the upstream provenance, architecture and
  digest, then update this document and the image catalog in one commit.
- Keep the old digest available while supported installations may still depend
  on it.
- Mirror only third-party runtime dependencies. Noryx CE and Enterprise images
  are never published through this mechanism.
