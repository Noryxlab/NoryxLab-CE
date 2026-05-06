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
- Jupyter/Python packages:
  - `ipywidgets`
  - `jupyterlab-git`
  - `jupyterlab-lsp`
  - `python-lsp-server[all]`
  - `jupyter-resource-usage`
  - `jupyterlab-code-formatter`
  - `black`, `isort`, `ruff`

Auto-update:

- At workspace startup, Noryx runs `noryx-sync-ide-tooling` once per day per user profile.
- Disable with env var `NORYX_AUTO_UPDATE_IDE=0`.

Target image:

- `harbor.lan/noryx-environments/noryx-python:0.2.2`

Build with Noryx API (`/api/v1/builds`) using:

- `dockerfilePath`: `environments/noryx-python/Dockerfile`
- `contextPath`: `` (empty)
- `destinationImage`: `harbor.lan/noryx-environments/noryx-python:0.2.2`
