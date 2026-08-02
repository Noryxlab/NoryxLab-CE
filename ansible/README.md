# Ansible Bootstrap (Noryx CE)

## Scope

Bootstrap a Noryx CE host with:

- base OS packages
- k3s
- helm
- longhorn CSI
- observability baseline (`kube-prometheus-stack`, `loki`, `promtail`)
- baseline services in Kubernetes (`postgres`, `keycloak`, `minio`, `noryx-backend`, `noryx-frontend`)

The `common` role also installs operator tooling on target node:

- `jq`
- `rsync`
- `git`

## Preamble

External services should exist before running a production-like bootstrap:

- Harbor or another OCI registry
- Dockerbuild VM or another build/push runner
- optional S3-compatible object store for backups/artifacts/staging

See `docs/INFRA_PREREQUISITES.md`.

## Service Account Model

Ansible uses a dedicated account on target host:

- user: `noryxops`
- SSH key auth only
- sudo configured for non-interactive automation

## Files

- `inventory/hosts.ini`: default demo target hosts
- `inventory/example-edge.ini`: generic HAProxy edge inventory template
- `inventory/example-s3.ini`: generic S3 manager inventory template
- `inventory/example-worker.ini`: generic worker-only inventory fragment
- `inventory/example-cluster.ini`: generic master + worker inventory template
- `playbooks/bootstrap-demo.yml`: main CE bootstrap playbook
- `playbooks/bootstrap-edge.yml`: HAProxy edge bootstrap playbook
- `playbooks/bootstrap-s3.yml`: S3 manager bootstrap playbook
- `playbooks/bootstrap-worker.yml`: k3s worker join playbook
- `clients/demo.yaml`: demo variables

Customer-specific inventories, domains, IP addresses, and secrets must stay out
of the public CE repository. Keep them in a private operations repository or in
operator-local files.

## Run

```bash
cd ansible
ansible-playbook playbooks/bootstrap-demo.yml -e @../clients/demo.yaml
```

For a HAProxy edge, copy `inventory/example-edge.ini` to a private inventory and
replace `CHANGE_ME`:

```bash
cd ansible
ansible-playbook -i /path/to/private-edge.ini playbooks/bootstrap-edge.yml
```

For an optional S3 manager VM, copy `inventory/example-s3.ini` to a private
inventory and run:

```bash
cd ansible
ansible-playbook -i /path/to/private-s3.ini playbooks/bootstrap-s3.yml \
  -e s3_manager_root_password='<strong-password>'
```

The S3 manager is intended for backups, artifacts, staging, and optional
non-HDS local buckets. HDS datasets stay on external HDS-certified S3 buckets.
Do not place the S3 manager data directory on the master/control-plane disk.

Before k3s/Traefik exists, the edge listens on `80/443` and returns a clear
placeholder on HTTP. After k3s is installed, pass Traefik NodePorts:

```bash
ansible-playbook -i /path/to/private-edge.ini playbooks/bootstrap-edge.yml \
  -e haproxy_backend_http_nodeport=<traefik-http-nodeport> \
  -e haproxy_backend_https_nodeport=<traefik-https-nodeport>
```

For a worker VM, use an inventory that contains both `noryx_master` and
`noryx_workers`. The playbook reads the k3s node token from the master through
Ansible delegation and joins each worker as a k3s agent:

```bash
cd ansible
ansible-playbook -i /path/to/private-cluster.ini playbooks/bootstrap-worker.yml \
  -e @/path/to/private-vars.yaml
```

The worker playbook also configures Harbor hostname resolution and containerd
registry trust before joining the cluster.

## One-Time Host Preparation

From your laptop:

```bash
ssh-keygen -t ed25519 -C "noryxops" -f ~/.ssh/id_ed25519_noryxops
scp ~/.ssh/id_ed25519_noryxops.pub noryxlab-master:/tmp/noryxops.pub
scp scripts/vm/create-noryxops.sh noryxlab-master:/tmp/create-noryxops.sh
ssh -t noryxlab-master 'sudo bash /tmp/create-noryxops.sh /tmp/noryxops.pub'
```

Validation:

```bash
ssh -i ~/.ssh/id_ed25519_noryxops noryxops@<host> 'sudo -n true && echo ok'
```

## Notes

- Password variables in `clients/demo.yaml` are placeholders and must be changed.
- Harbor integration variables are in `clients/demo.yaml`:
  - `harbor_registry_host`
  - `harbor_registry_ip`
  - `harbor_registry_insecure_skip_verify`
- Longhorn variables are in `clients/demo.yaml`:
  - `longhorn_chart_version` (empty = latest chart)
  - `longhorn_default_replica_count` (`1` for single-node lab)
- Observability variables are in `clients/demo.yaml`:
  - `observability_enabled`
  - `observability_namespace`
  - `observability_loki_retention_hours` (default `744` = 31 days)
  - `observability_storage_class`
  - `observability_prometheus_size`
  - `observability_grafana_size`
  - `observability_loki_size`
