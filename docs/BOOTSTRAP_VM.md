# VM Bootstrap (Noryx CE)

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

## Deploy baseline platform

```bash
kubectl apply -k deploy/k8s/base
kubectl -n noryx-ce get pods
```

## Notes

- Harbor remains external.
- MinIO starts in-cluster for V1 and can be externalized later.
