# VM Bootstrap (Noryx CE)

## Preamble (mandatory)

Prepare two external VMs first:

- Harbor VM (registry)
- Dockerbuild VM (build/push)

Reference: `docs/INFRA_PREREQUISITES.md`.

## Minimum target

- Ubuntu 24.04 LTS
- 8 vCPU
- 32 GB RAM
- 300 GB SSD
- Public DNS: `demo.noryxlab.ai`

## Network

Open inbound ports:

- 22/tcp
- 80/tcp
- 443/tcp

## Base install

```bash
curl -sfL https://get.k3s.io | sh -
sudo kubectl get nodes
```

## Sudo requirement for Ansible

Target automation user must have sudo rights.

Recommended model:

- create dedicated service account `noryxops`
- enable SSH key auth
- configure `noryxops` in `/etc/sudoers.d/` with `NOPASSWD` for automation

Helper script:

```bash
sudo bash scripts/vm/create-noryxops.sh /tmp/noryxops.pub
```

## Deploy baseline platform

```bash
kubectl apply -k deploy/k8s/base
kubectl -n noryx-ce get pods
```

## Notes

- Harbor remains external.
- MinIO starts in-cluster for V1 and can be externalized later.
