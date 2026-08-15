# Frontend source

`index.html`, `favicon.svg`, and `version.json` are the editable frontend sources.
The Kubernetes ConfigMap remains committed as a generated deployment artifact for
Kustomize and the EE branding renderer.

After changing a source file, synchronize and validate the manifest:

```sh
frontend/sync-manifest.rb
frontend/sync-manifest.rb --check
```

Do not edit the ConfigMap data blocks in `deploy/k8s/base/noryx-frontend.yaml`
directly.
