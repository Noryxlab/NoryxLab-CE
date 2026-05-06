FROM python:3.12-slim-bookworm

ENV DEBIAN_FRONTEND=noninteractive \
    OPENVSCODE_VERSION=1.99.3 \
    OPENVSCODE_DATA_DIR=/home/noryx/.openvscode-server \
    NORYX_AUTO_UPDATE_IDE=1 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
    ca-certificates \
    curl \
    git \
    sudo \
    tini \
 && rm -rf /var/lib/apt/lists/*

RUN python3 -m pip install --no-cache-dir \
    jupyterlab==4.2.5 \
    notebook==7.2.2 \
    ipywidgets==8.1.5 \
    jupyterlab-git==0.50.2 \
    jupyterlab-lsp==5.1.0 \
    python-lsp-server[all]==1.12.0 \
    jupyter-resource-usage==1.1.1 \
    jupyterlab-code-formatter==2.2.1 \
    black==24.10.0 \
    isort==5.13.2 \
    ruff==0.8.1

RUN mkdir -p /opt/openvscode-server \
 && curl -fsSL "https://github.com/gitpod-io/openvscode-server/releases/download/openvscode-server-v${OPENVSCODE_VERSION}/openvscode-server-v${OPENVSCODE_VERSION}-linux-x64.tar.gz" \
    | tar -xz --strip-components=1 -C /opt/openvscode-server \
 && ln -sf /opt/openvscode-server/bin/openvscode-server /usr/local/bin/openvscode-server

RUN mkdir -p "${OPENVSCODE_DATA_DIR}/data/Machine" /mnt \
 && printf '%s\n' \
    '{' \
    '  "workbench.colorTheme": "Default Dark Modern",' \
    '  "python.defaultInterpreterPath": "/usr/local/bin/python3"' \
    '}' > "${OPENVSCODE_DATA_DIR}/data/Machine/settings.json"

RUN set -eux; \
    exts='ms-python.python ms-python.vscode-pylance ms-python.debugpy ms-toolsai.jupyter ms-toolsai.jupyter-keymap ms-toolsai.jupyter-renderers charliermarsh.ruff kilocode.Kilo-Code'; \
    for ext in $exts; do openvscode-server --install-extension "$ext" || true; done

RUN cat > /usr/local/bin/noryx-sync-ide-tooling <<'EOF'
#!/usr/bin/env bash
set -eu

PROFILE_DIR="${NORYX_PROFILE_DIR:-/home/noryx/.noryx-profile}"
STAMP_FILE="${PROFILE_DIR}/.ide-tooling-last-update"
TODAY="$(date +%F)"
if [ -f "${STAMP_FILE}" ] && [ "$(cat "${STAMP_FILE}" 2>/dev/null || true)" = "${TODAY}" ]; then
  echo "[ide-tooling] already updated today"
  exit 0
fi

mkdir -p "${PROFILE_DIR}"
echo "[ide-tooling] updating VS Code extensions"
exts='ms-python.python ms-python.vscode-pylance ms-python.debugpy ms-toolsai.jupyter ms-toolsai.jupyter-keymap ms-toolsai.jupyter-renderers charliermarsh.ruff kilocode.Kilo-Code'
for ext in $exts; do
  openvscode-server --install-extension "$ext" --force >/tmp/noryx-vscode-extensions.log 2>&1 || true
done

echo "[ide-tooling] updating Jupyter/python packages"
python3 -m pip install --disable-pip-version-check --no-cache-dir --upgrade \
  ipywidgets \
  jupyterlab-git \
  jupyterlab-lsp \
  python-lsp-server[all] \
  jupyter-resource-usage \
  jupyterlab-code-formatter \
  black \
  isort \
  ruff >/tmp/noryx-jupyter-extensions.log 2>&1 || true

echo "${TODAY}" > "${STAMP_FILE}"
echo "[ide-tooling] update complete"
EOF
RUN chmod +x /usr/local/bin/noryx-sync-ide-tooling

RUN useradd -m -s /bin/bash noryx \
 && usermod -aG sudo noryx \
 && echo 'noryx ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/90-noryx \
 && chmod 0440 /etc/sudoers.d/90-noryx \
 && mkdir -p /mnt /repos /datasets /home/noryx/.noryx-profile \
 && chown -R noryx:noryx /home/noryx /mnt /repos /datasets

EXPOSE 8888

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["python3", "-m", "jupyterlab", "--ip=0.0.0.0", "--port=8888", "--no-browser"]

USER noryx
