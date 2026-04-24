# Infrastructure Prerequisites (Preamble)

Noryx CE bootstrap depends on two external VMs:

- `harbor` VM: private container registry
- `dockerbuild` VM: image build and push runner

Without these two VMs, control-plane image pulls (`noryx-backend`, `noryx-frontend`) will fail in Kubernetes.

## In-cluster storage baseline

Noryx CE bootstrap installs Longhorn CSI for workspace PVCs.

Node prerequisites (handled by Ansible `common` + `longhorn` roles):

- `open-iscsi` package
- `iscsid` service enabled
- sufficient local disk capacity for workspace volumes

## Required baseline

### Harbor VM

- hostname/IP reachable from k3s node
- Harbor registry installed and running
- project created: `noryx-ce` (private recommended)
- robot account with `repository:pull` and `repository:push`

### Dockerbuild VM

- Docker installed and running
- network access to Harbor
- DNS/hosts resolution for Harbor hostname (for example `harbor.lan`)
- credentials to push in `noryx-ce` project

## Quick setup scripts

Use provided scripts on each VM:

- `scripts/vm/install-harbor-vm.sh`
- `scripts/vm/install-dockerbuild-vm.sh`

## Kubernetes node alignment

The k3s node must be able to:

- resolve Harbor hostname
- trust Harbor TLS (or temporary `insecure_skip_verify` in lab mode)
- pull with `imagePullSecrets` (`harbor-regcred`)

If using split namespaces (`noryx-ce` + `noryx-loads`):

- create `harbor-regcred` in both namespaces
- builds/workspaces run in `noryx-loads`, so missing secret there breaks runtime launches

## DNS note for in-cluster Kaniko builds

Kaniko build pods resolve registry hostnames using cluster DNS (CoreDNS), not host `/etc/hosts`.

If Harbor is exposed as `harbor.lan`, add it to CoreDNS NodeHosts:

- `192.168.1.106 harbor.lan`

Otherwise build jobs can fail with `lookup harbor.lan on 10.43.0.10:53: no such host`.
