# Jupyter Debian Base Environment

Base image for Noryx workspaces:

- Debian 12 slim
- Python3 + pip
- JupyterLab

Target image:

- `harbor.lan/noryx-environments/noryx-workspace-jupyter:0.1.0`

Build with Noryx API (`/api/v1/builds`) using:

- `dockerfilePath`: `environments/jupyter-debian/Dockerfile`
- `contextPath`: `` (empty)
- `destinationImage`: `harbor.lan/noryx-environments/noryx-workspace-jupyter:0.1.0`
