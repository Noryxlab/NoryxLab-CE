FROM python:3.12-slim-bookworm

ENV DEBIAN_FRONTEND=noninteractive \
    OPENVSCODE_VERSION=1.99.3 \
    OPENVSCODE_DATA_DIR=/home/workspace/.openvscode-server \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git \
    tini \
 && rm -rf /var/lib/apt/lists/*

RUN python3 -m pip install --no-cache-dir \
    jupyterlab==4.2.5 \
    notebook==7.2.2

RUN mkdir -p /opt/openvscode-server \
 && curl -fsSL "https://github.com/gitpod-io/openvscode-server/releases/download/openvscode-server-v${OPENVSCODE_VERSION}/openvscode-server-v${OPENVSCODE_VERSION}-linux-x64.tar.gz" \
    | tar -xz --strip-components=1 -C /opt/openvscode-server \
 && ln -sf /opt/openvscode-server/bin/openvscode-server /usr/local/bin/openvscode-server

RUN mkdir -p "${OPENVSCODE_DATA_DIR}/data/Machine" /workspace \
 && printf '%s\n' \
    '{' \
    '  "workbench.colorTheme": "Default Dark Modern",' \
    '  "python.defaultInterpreterPath": "/usr/local/bin/python3"' \
    '}' > "${OPENVSCODE_DATA_DIR}/data/Machine/settings.json"

RUN openvscode-server --install-extension ms-python.python \
 || true

EXPOSE 8888

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["python3", "-m", "jupyterlab", "--ip=0.0.0.0", "--port=8888", "--no-browser"]
