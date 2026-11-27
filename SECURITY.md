# Security policy

## Reporting a vulnerability

Email **security@noryxlab.ai**. Please do not open a public issue for a
security problem.

Tell us what you can of: the component and version, what an attacker can do
with it, and how to reproduce it. A rough report sent early is more useful
than a polished one sent late — if you are unsure whether something counts,
send it.

What we commit to, and we have kept it deliberately modest because an
unmeetable promise in this file is worse than no file:

| | |
| --- | --- |
| Acknowledgement | within 3 working days |
| First assessment — is it a vulnerability, how severe, does it affect released versions | within 10 working days |
| Fix or a stated plan | agreed with you, by severity |
| Credit | yours unless you ask otherwise |

We will agree a disclosure date with you rather than impose one. If we
disagree about severity we will say so and explain why, rather than go quiet.

## What is in scope

This repository (NoryxLab Community Edition) and the container images built
from it. Also in scope: the Enterprise edition, which is not published here —
report those the same way.

Out of scope: the third-party components the platform deploys (PostgreSQL,
Keycloak, MinIO, Traefik, Longhorn, k3s and the rest). Report those upstream.
If a vulnerability in one of them is made reachable *by how Noryx configures
or deploys it*, that is in scope and we want to hear about it.

Also out of scope: findings from an automated scanner pasted without a
reachable path. We read them, but a report that says "this library has a CVE"
without saying what it lets somebody do takes the same time to assess as a
real one.

## Which versions get fixes

The platform is at 0.x and ships as a rolling release: fixes land in the next
version, and installations upgrade by moving to it. There is no long-term
support branch, and we would rather say so than imply a maintenance window we
do not operate.

An installation that cannot upgrade immediately can ask us for the specific
patch and the mitigation that holds until it is applied.

## What the platform does about vulnerabilities on its own

Stated here because reviewers ask, and because some of it is visible to an
operator rather than to us:

- **Images are scanned on push.** The registry scans every image with Trivy
  as it is published, and the platform reads those counts back: an
  environment's vulnerability figures are shown in the catalogue beside it,
  per image, with the date of the scan.
- **A scan is never read as a verdict.** The platform does not refuse to start
  a workspace because a scanner found a high-severity CVE, and it does not
  claim an image is clean when the registry has not scanned it — "not scanned"
  is shown as what it is.
- **What actually ran is recorded.** Every job, application and workspace
  records the image *digest* it executed, not the tag, so a question about a
  vulnerable image can be answered with the runs that used it.

## Hardening the deployment

Most of what protects data in practice is deployment, not code: network egress
rules, who holds which right, whether downloads are permitted, where regulated
data is stored. Those are documented separately:

- [How data leaves a workspace](docs/HOW_DATA_LEAVES_A_WORKSPACE.md) — every
  route out, and which ones are closed;
- [Workload network isolation](docs/WORKLOAD_NETWORK_ISOLATION.md);
- [RBAC model](docs/RBAC_MODEL.md);
- [Writing beside regulated data](docs/WRITING_BESIDE_REGULATED_DATA.md).
