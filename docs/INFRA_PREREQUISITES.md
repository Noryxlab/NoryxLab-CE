# Infrastructure Prerequisites (Preamble)

Noryx CE bootstrap depends on two external VMs:

- `harbor` VM: private container registry
- `dockerbuild` VM: image build and push runner

Without these two VMs, `noryx-api` image pull will fail in Kubernetes.

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
