# VSCode Python 3.12 Environment

Base image for Noryx VSCode workspaces:

- OpenVSCode Server (web IDE)
- Python 3.12
- Git
- Default dark theme

Target image:

- `harbor.lan/noryx-environments/noryx-workspace-vscode:0.1.0`

Build with Noryx API (`/api/v1/builds`) using:

- `dockerfilePath`: `environments/vscode-python312/Dockerfile`
- `contextPath`: `` (empty)
- `destinationImage`: `harbor.lan/noryx-environments/noryx-workspace-vscode:0.1.0`
