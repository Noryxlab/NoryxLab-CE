# Ansible Bootstrap (Noryx CE)

## Scope

Bootstrap one CE demo host with:

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

External services must exist before running this playbook:

- Harbor VM (registry)
- Dockerbuild VM (build and push)

See `docs/INFRA_PREREQUISITES.md`.

## Service account model

Ansible uses a dedicated account on target host:

- user: `noryxops`
- SSH key auth only
- sudo configured for non-interactive automation

## Files

- `inventory/hosts.ini`: default target hosts
- `playbooks/bootstrap-demo.yml`: main playbook
- `clients/demo.yaml`: environment-specific variables

## Run

```bash
cd ansible
ansible-playbook playbooks/bootstrap-demo.yml -e @../clients/demo.yaml
```

For the Example/Noryx EE bootstrap target:

```bash
cd ansible
ansible-playbook -i inventory/example.ini playbooks/bootstrap-demo.yml -e @../clients/example-temp.yaml
```

Use `../clients/example-temp.yaml` while `datalab-example.noryxlab.ai` is the
temporary public name. Switch to `../clients/example.yaml` only when the final
`datalab.example.fr` DNS and TLS are ready.

For the Example HAProxy edge:

```bash
cd ansible
ansible-playbook -i inventory/example-edge.ini playbooks/bootstrap-edge.yml
```

For the Example Harbor and dockerbuild VMs, use:

```bash
scripts/vm/install-harbor-vm.sh
scripts/vm/install-dockerbuild-vm.sh
```

Current Example inventory files:

- `inventory/example-edge.ini`: `noryx-edge` / `127.0.0.1`
- `inventory/example-harbor.ini`: `noryx-registry` / `127.0.0.1`
- `inventory/example-dockerbuild.ini`: `noryx-dockerbuild` / `127.0.0.1`
- `inventory/example.ini`: `noryx-master` / `127.0.0.1`

Before k3s/Traefik exists, the edge listens on `80/443` and returns a clear
placeholder on HTTP. After k3s is installed, pass Traefik NodePorts:

```bash
ansible-playbook -i inventory/example-edge.ini playbooks/bootstrap-edge.yml \
  -e haproxy_backend_http_nodeport=<traefik-http-nodeport> \
  -e haproxy_backend_https_nodeport=<traefik-https-nodeport>
```

## One-time host preparation

From your laptop:

```bash
ssh-keygen -t ed25519 -C "noryxops" -f ~/.ssh/id_ed25519_noryxops
scp ~/.ssh/id_ed25519_noryxops.pub noryxlab-master:/tmp/noryxops.pub
scp scripts/vm/create-noryxops.sh noryxlab-master:/tmp/create-noryxops.sh
ssh -t noryxlab-master 'sudo bash /tmp/create-noryxops.sh /tmp/noryxops.pub'
```

Validation:

```bash
ssh -i ~/.ssh/id_ed25519_noryxops noryxops@CHANGE_ME 'sudo -n true && echo ok'
```

## Notes

- Host alias used by default: `noryxlab-master` (`CHANGE_ME`)
- Domain for this environment: `datalab.example.local`
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
