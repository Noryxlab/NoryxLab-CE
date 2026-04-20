# Ansible Bootstrap (Noryx CE)

## Scope

Bootstrap one CE demo host with:

- base OS packages
- k3s
- helm
- baseline services in Kubernetes (`postgres`, `keycloak`, `minio`, `noryx-api`)

## Files

- `inventory/hosts.ini`: target hosts
- `playbooks/bootstrap-demo.yml`: main playbook
- `clients/demo.yaml`: environment-specific variables

## Run

```bash
cd ansible
ansible-playbook playbooks/bootstrap-demo.yml -e @../clients/demo.yaml
```

If `sudo` requires a password on target host:

```bash
ansible-playbook playbooks/bootstrap-demo.yml -e @../clients/demo.yaml -K
```

## Notes

- Host alias used by default: `noryxlab-master` (`192.168.1.140`)
- Domain for this environment: `datalab.noryxlab.ai`
- Password variables in `clients/demo.yaml` are placeholders and must be changed.
