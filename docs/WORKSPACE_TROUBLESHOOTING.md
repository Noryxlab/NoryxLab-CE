# Workspace Troubleshooting (Blank Page / Open Flow)

This runbook targets `datalab.example.local` with path-based workspace routing:

- `/workspaces/<workspaceID>/...`
- no wildcard DNS required

## 1) Confirm deployed versions

```bash
curl -sk https://datalab.example.local/ | rg "FRONT_VERSION|ce-web"
curl -sk https://datalab.example.local/swagger/openapi.yaml | rg "^\\s*version:"
```

Expected for current fix line:

- front `ce-web-0.6.19+`
- back `0.5.21+`

## 2) Check workspace is running

```bash
ssh noryxlab-master 'KUBECONFIG=/home/stef/.kube/config kubectl -n noryx-loads get pods,svc -o wide'
ssh noryxlab-master 'KUBECONFIG=/home/stef/.kube/config kubectl -n noryx-loads get pvc -o wide'
```

If pod/service/PVC are missing, open will fail regardless of UI state.

Longhorn health check:

```bash
ssh noryxlab-master 'KUBECONFIG=/home/stef/.kube/config kubectl -n longhorn-system get pods'
```

## 3) Validate Jupyter HTML from workspace URL

Use the exact `accessUrl` returned by `GET /api/v1/workspaces`:

```bash
curl -sk "<BASE><accessUrl>" | head -n 5
```

Expected: HTML containing `JupyterLab` and `jupyter-config-data`.

## 4) Validate mandatory SSO protection

An anonymous request, including one with a legacy workspace token, must fail:

```bash
curl -sk "<BASE>/workspaces/<workspaceID>/lab?reset&token=<legacy-token>" \
  -o /tmp/anonymous-response.json -w "%{http_code}\n"
```

Expected: HTTP `401`. An authenticated project member with launch rights must
receive HTTP `200`.

## 5) Typical symptoms and causes

- Home page replaced by white page when clicking `Open`
  - front regression opening in same tab
  - fixed in `ce-web-0.6.17+`
- New tab opens but stays blank
  - session/cookie propagation issue in browser
  - validate the authenticated session and project RBAC (step 4)
- `workspace not found` on `/workspaces/<id>/...`
  - in-memory metadata reset after back restart
  - trigger `GET /api/v1/workspaces` authenticated to re-sync runtime records
- `403 insufficient role for workspace access/deletion`
  - caller is not `editor|admin` on workspace project

## 6) Browser-side checks

- hard refresh (`Cmd+Shift+R`)
- allow popups for `datalab.example.local`
- check DevTools Network for first failing request under `/workspaces/<workspaceID>/...`
- if first failing response is JSON, read its `error` value and map with sections above

## 7) The workspace reports "failed" seconds after launch

The pod is created, the container starts, and the workspace is `failed` within
a minute. Nothing in the backend log says why, because nothing went wrong on
the platform side: the bootstrap script exited non-zero and the pod, which has
`restartPolicy: Never`, went to phase `Failed`.

The bootstrap's own output is the answer, and it is only readable while the pod
exists - a validator run or a person clicking again will delete it:

```bash
ssh noryxlab-master 'sudo kubectl -n noryx-loads get pods | grep ^wks-'
ssh noryxlab-master 'sudo kubectl -n noryx-loads logs <pod> --all-containers --timestamps'
ssh noryxlab-master 'sudo kubectl -n noryx-loads get pod <pod> \
  -o jsonpath="{range .status.containerStatuses[*]}{.state}{end}"'
```

The container status carries the exit code, which narrows it immediately:

- **exit 2, "Permission denied" on a path under the profile mount.** Workspaces
  ran as root for a period, and what that root created inside the per-user
  profile volume stayed `root:root 0755`. The volume root is `0777`, so most of
  the tree is writable and only those directories are not. Three volumes on
  EMSE carried the residue on 2026-10-01, two belonging to people who could no
  longer start a workspace at all - their only symptom was the word "failed".

  The bootstrap now takes the tree back with `sudo chown` and tolerates the
  write when it cannot, so this should not recur. An older volume can be
  repaired directly by running a `busybox` pod that mounts the claim and
  `chown -R 1000:1000` it - 1000:1000 is the `noryx` user every shipped image
  runs as.

- **exit 127, "not found" naming a whole command line.** The launch command was
  passed as a single argv word, so the container looked for a program with that
  entire name. The platform now interposes a shell for a single word containing
  whitespace; an older caller sending `{"args":["python3 -m http.server 9000"]}`
  should send the words separately.

Checking ownership inside a profile volume, without touching it:

```bash
# A pod that mounts the claim read-only and prints what it finds.
sudo kubectl -n noryx-loads run profil-audit --rm -it --restart=Never \
  --image=busybox:1.36 \
  --overrides='{"spec":{"containers":[{"name":"main","image":"busybox:1.36",
    "command":["sh","-c","ls -ldn /p /p/jupyter/config"],
    "volumeMounts":[{"name":"p","mountPath":"/p","readOnly":true}]}],
    "volumes":[{"name":"p","persistentVolumeClaim":{"claimName":"profile-<user>","readOnly":true}}]}}'
```

Anything owned by `0 0` under the profile mount is the residue above.

**Workloads live in `noryx-loads`, not `noryx`.** The pod is named
`wks-<short id>` and carries neither the workspace's name nor the project's, so
grepping for either finds nothing. Three separate attempts to catch this
failure looked in the control-plane namespace and concluded, wrongly, that no
pod was ever created.
