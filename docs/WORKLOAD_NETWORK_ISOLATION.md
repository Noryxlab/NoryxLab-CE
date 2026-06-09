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
- egress to public IP addresses.

The public-egress policy blocks:

- Kubernetes service and pod networks;
- RFC1918 private networks;
- carrier-grade NAT addresses;
- link-local addresses.

This blocks direct access from user workloads to the Kubernetes API, Noryx
backend, Keycloak, PostgreSQL, MinIO, Harbor, other workloads, and private LAN
services.

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
