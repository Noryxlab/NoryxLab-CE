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

Validation:

```bash
ssh -i ~/.ssh/id_ed25519_noryxops noryxops@192.168.1.140 'sudo -n kubectl get ns'
```

If you use `~/.ssh/config` with a host alias (for example `noryxlab-master`), ensure it does not force the `stef` key for `noryxops` connections.
Recommended:

```sshconfig
Host noryxlab-master
  HostName 192.168.1.140
  User stef
  IdentityFile ~/.ssh/id_ed25519_noryx_vm
  IdentitiesOnly yes

Host noryxlab-master-ops
  HostName 192.168.1.140
  User noryxops
  IdentityFile ~/.ssh/id_ed25519_noryxops
  IdentitiesOnly yes
```

## Deploy baseline platform

```bash
kubectl apply -k deploy/k8s/base
kubectl -n noryx-ce get pods
```

Longhorn is installed by Ansible bootstrap (`role: longhorn`).
Validation:

```bash
kubectl -n longhorn-system get pods
kubectl get sc longhorn
```

## Enable TLS for datalab.noryxlab.ai

Apply Traefik ACME config:

```bash
kubectl apply -f deploy/k8s/infra/traefik/letsencrypt-helmchartconfig.yaml
kubectl -n kube-system rollout restart deploy/traefik
```

Requirements:

- `datalab.noryxlab.ai` resolves to your public IP
- router/NAT forwards TCP 80 and 443 to `192.168.1.140`

## Notes

- Harbor remains external.
- MinIO starts in-cluster for V1 and can be externalized later.
- VM helper scripts install baseline tooling (`jq`, `rsync`, `python3`) for operations.
