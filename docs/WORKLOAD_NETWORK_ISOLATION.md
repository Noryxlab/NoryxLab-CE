# Workload network isolation

Noryx isolates user workloads in the `noryx-loads` namespace with Kubernetes
NetworkPolicies.

The baseline applies to workspaces, jobs, applications, dashboards, and ad hoc
pods created through the Noryx runtime. Image build jobs are excluded because
they require private Harbor access.

## Default policy

Every user workload receives:

```text
noryx.io/network-isolation=user-workload
```

Selected pods are denied all ingress and egress by default. Explicit policies
then allow:

- ingress from the Noryx backend reverse proxy;
- UDP and TCP DNS queries to CoreDNS;
- egress to the backend's API port and to the project's data services;
- egress to public IP addresses, **for the profiles that admit it**.

The public-egress policy blocks:

- Kubernetes service and pod networks;
- RFC1918 private networks;
- carrier-grade NAT addresses;
- link-local addresses.

This blocks direct access from user workloads to the Kubernetes API, Noryx
backend, Keycloak, PostgreSQL, MinIO, Harbor, other workloads, and private LAN
services.

## The egress profile decides the internet

Every workload also carries the profile it runs under:

```text
noryx.io/egress-profile=<isolated|internal-only|package-access|approved-egress|hds-regulated|unrestricted>
```

The public-egress policy selects the three open profiles - `unrestricted`,
`package-access`, `approved-egress`. A pod labelled `isolated`,
`internal-only` or `hds-regulated` matches it and nothing else, so the
default-deny above governs its egress: DNS, the platform API and its data
services stay reachable, and the internet does not.

**What decides the label** is `egress.enforcement`, a platform setting:

| Value | A project holding regulated data | Every other project |
| --- | --- | --- |
| `off` | public egress | public egress |
| `regulated` (default) | closed | public egress |
| `strict` | closed | closed |

Regulated means what it means everywhere else on this platform: an attached
dataset classified `hds`, or an attached extract read from one. Both doors are
checked, because an extract is attached without its dataset.

**Two profiles are recorded and not applied.** `package-access` and
`approved-egress` name domains, and a NetworkPolicy selects addresses and
ports. Narrowing them needs the egress gateway of ADR-023; until it exists
they are listed as open rather than claimed as narrow.

**What a closed workload cannot do**, and this is the part to say before
somebody meets it: `pip install` from PyPI, `apt-get` from a public mirror,
`git clone` from GitHub or Azure DevOps, and any call to a model endpoint
outside the site. The workspace still reaches the platform's own API, so the
Enterprise assistant keeps working. Packages and repositories are what an
internal mirror is for - see ADR-023's rollout.

**A workload is unrestricted for its first seconds.** Measured on
2026-10-30: a pod labelled `hds-regulated` reached `pypi.org` at t+0s and was
refused at t+10s and t+30s. The policy controller programs a pod's firewall
chain after the pod has an address, so there is a window between the two.

It does not matter for a workspace, where the person's shell starts later. It
matters for a job, which runs its command immediately: a job written to
exfiltrate in its first second would succeed. Closing that window means
holding user code until the policy is active - a start-up delay on every
closed workload, and a decision about what to do when the probe never closes -
and it is not done yet. Say it plainly to anyone who asks what the control
covers.

**Already-running workloads keep what they had.** A policy is evaluated
against live pods, so a second rule admits pods carrying no profile label at
all: a workspace started before the upgrade is not cut off mid-session, and
gets its profile the next time it starts.

## Desktop sessions and the clipboard

Slicer and ParaView are desktops served over VNC, and neither web client
carries a file transfer, so the clipboard is the only channel that moves bytes
to the viewer's machine. It is closed in that direction and open in the other:
`-SendCutText 0` on KasmVNC, `-noclipboard -noprimary` on x11vnc. Pasting a
path or an expression into the application still works.

## ServiceAccount protection

User workload pods are created with:

```yaml
automountServiceAccountToken: false
```

They therefore receive no Kubernetes API token.

## Private resource exceptions

Private Git repositories, datasources, APIs, or S3 endpoints are blocked by
default. Access must be introduced through a reviewed, narrow NetworkPolicy
exception scoped to:

- the required workload or project label;
- the exact destination CIDR;
- the required TCP or UDP ports.

Do not add a general RFC1918 allow rule.

Standard Kubernetes NetworkPolicy cannot filter destinations by DNS name.
Provider endpoints with changing IP ranges require a CNI supporting FQDN
policies or a controlled egress proxy.

## Deployment

Policies are defined in:

```text
deploy/k8s/base/noryx-loads-network-policies.yaml
```

Apply them through the base Kustomization:

```sh
kubectl apply -k deploy/k8s/base
```

## Validation

Check workload protection:

```sh
kubectl -n noryx-loads get pod <pod> \
  -o jsonpath='{.metadata.labels.noryx\.io/network-isolation}{"\n"}'
kubectl -n noryx-loads get pod <pod> \
  -o jsonpath='{.spec.automountServiceAccountToken}{"\n"}'
kubectl -n noryx-loads exec <pod> -- \
  test ! -e /var/run/secrets/kubernetes.io/serviceaccount/token
```

Expected values are `user-workload`, `false`, and a successful token absence
check.

From a test workspace:

- public DNS and HTTPS must work;
- Noryx internal service IPs must be unreachable;
- another workload pod IP must be unreachable;
- access through the authenticated Noryx workspace proxy must still work.

## CNI requirement

The cluster CNI must enforce Kubernetes NetworkPolicy. Applying policy objects
on a CNI without enforcement provides no isolation. This must be verified after
every cluster networking change.
