# Ansible Bootstrap (Noryx CE)

## Scope

Bootstrap one CE demo host with:

- base OS packages
- k3s
- helm
- longhorn CSI
- baseline services in Kubernetes (`postgres`, `keycloak`, `minio`, `noryx-api`)

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

- `inventory/hosts.ini`: target hosts
- `playbooks/bootstrap-demo.yml`: main playbook
- `clients/demo.yaml`: environment-specific variables

## Run

```bash
cd ansible
ansible-playbook playbooks/bootstrap-demo.yml -e @../clients/demo.yaml
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
ssh -i ~/.ssh/id_ed25519_noryxops noryxops@192.168.1.140 'sudo -n true && echo ok'
```

## Notes

- Host alias used by default: `noryxlab-master` (`192.168.1.140`)
- Domain for this environment: `datalab.noryxlab.ai`
- Password variables in `clients/demo.yaml` are placeholders and must be changed.
- Harbor integration variables are in `clients/demo.yaml`:
  - `harbor_registry_host`
  - `harbor_registry_ip`
  - `harbor_registry_insecure_skip_verify`
- Longhorn variables are in `clients/demo.yaml`:
  - `longhorn_chart_version` (empty = latest chart)
  - `longhorn_default_replica_count` (`1` for single-node lab)
