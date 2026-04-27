# Noryx Python Unified Environment

Single base image for Noryx workloads:

- Python 3.12
- JupyterLab
- OpenVSCode Server
- Git
- user `noryx` (default) with `sudo` enabled

Target image:

- `harbor.lan/noryx-environments/noryx-python:0.2.2`

Build with Noryx API (`/api/v1/builds`) using:

- `dockerfilePath`: `environments/noryx-python/Dockerfile`
- `contextPath`: `` (empty)
- `destinationImage`: `harbor.lan/noryx-environments/noryx-python:0.2.2`
