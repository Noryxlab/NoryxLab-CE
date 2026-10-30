# Public image mirrors

Noryx CE installations mirror their runtime dependencies into the customer's
own registry before deployment. The catalog is
[`deploy/images/essential-images.txt`](../deploy/images/essential-images.txt).

## MinIO — no longer a mirror

**MinIO stopped distributing community container images.** On 2026-10-30
`docker pull minio/minio` and `quay.io/minio/minio` both answer "access
denied" for the whole repository, not for one release; and its last
open-source release is `RELEASE.2025-10-15T17-29-55Z`, with nothing published
since. The community edition moved behind their commercial product.

So there is nothing left to mirror, and the image this platform runs is
**built from MinIO's own AGPL-3.0 source** by
[`scripts/ops/build-minio-image.sh`](../scripts/ops/build-minio-image.sh),
which states what it changes and why. In short: a current Go toolchain, which
removes the standard library's accumulated advisories; no `mc`, which the
server pod never execs; two dependencies raised past their advisories; and a
distroless base. Measured with trivy, critical and high: 7 and 128 for the
published image, 0 and 32 for this one.

A build is not a mirror, and this section says so rather than leaving a reader
to assume provenance we no longer have. What we build, we answer for.

The earlier mirror stays published for installations that still depend on it:

```text
ghcr.io/noryxlab/noryx-ce-minio@sha256:5c1e0c9ca1c34b1f2a0f66345f5e2acd2efe98b3aa91089b5cf8ff08b593cda7
```

It is a byte-for-byte copy of `minio/minio:RELEASE.2025-02-18T16-25-55Z`,
mirrored on 2026-10-09 when upstream stopped accepting anonymous pulls for it,
and published only so a new installation can bootstrap its own registry. It
contains no Noryx source code, images, configuration, credentials or customer
data. MinIO is licensed under AGPL-3.0; its project is
<https://github.com/minio/minio>.

**What has not been decided** is where the object store goes next. Building a
release upstream has abandoned is a real improvement and a dead end: there
will be no further fixes to compile. The three options are a commercial
subscription, carrying this build knowing it is frozen, and another object
store. None of them is a maintenance task.

## Maintenance policy

- Pin all consumers to a manifest digest, never a mutable tag. `postgres:16`
  broke that rule for a year: a floating tag whose mirror held 16.13 while
  upstream had moved to 16.15, and nothing said so. It is mirrored under
  `postgres:16.15` as well, and that is what the deployments name.
- Before replacing a mirror, verify the upstream provenance, architecture and
  digest, then update this document and the image catalog in one commit.
- Keep the old digest available while supported installations may still depend
  on it.
- Mirror only third-party runtime dependencies. Noryx CE and Enterprise images
  are never published through this mechanism.
