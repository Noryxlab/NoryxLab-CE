# Noryx Python Unified Environment

Single base image for Noryx workloads:

- Python 3.12
- JupyterLab
- OpenVSCode Server
- Git
- user `noryx` (default) with `sudo` enabled

Bundled Python/Data Science tooling:

- VS Code extensions:
  - `ms-python.python`
  - `ms-python.vscode-pylance`
  - `ms-python.debugpy`
  - `ms-toolsai.jupyter`
  - `ms-toolsai.jupyter-keymap`
  - `ms-toolsai.jupyter-renderers`
  - `charliermarsh.ruff`
  - `kilocode.Kilo-Code` (best effort; may depend on extension registry availability)
- Jupyter/Python packages:
  - `ipywidgets`
  - `jupyterlab-git`
  - `jupyterlab-lsp`
  - `python-lsp-server[all]`
  - `jupyter-resource-usage`
  - `jupyterlab-code-formatter`
  - `black`, `isort`, `ruff`

Auto-update:

- At workspace startup, Noryx can run `noryx-sync-ide-tooling` once per day per user
  profile. It is **off by default**: it reinstalls the extensions from the
  marketplace while the person is waiting for their editor, needs an outbound
  route the installation may not have, and makes a pinned image produce
  different tooling from one day to the next.
- Enable with env var `NORYX_AUTO_UPDATE_IDE=1`.

Target image:

- `harbor.example.local/noryx-environments/noryx-python:0.2.3`

Build with Noryx API (`/api/v1/builds`) using:

- `dockerfilePath`: `environments/noryx-python/Dockerfile`
- `contextPath`: `` (empty)
- `destinationImage`: `harbor.example.local/noryx-environments/noryx-python:0.2.3`
