FROM python:3.12-slim-bookworm

ENV DEBIAN_FRONTEND=noninteractive \
    OPENVSCODE_VERSION=1.99.3 \
    OPENVSCODE_DATA_DIR=/home/noryx/.openvscode-server \
    NORYX_WORKSPACE_IDE=vscode \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl git sudo tini \
 && rm -rf /var/lib/apt/lists/*

RUN mkdir -p /opt/openvscode-server \
 && curl -fsSL "https://github.com/gitpod-io/openvscode-server/releases/download/openvscode-server-v${OPENVSCODE_VERSION}/openvscode-server-v${OPENVSCODE_VERSION}-linux-x64.tar.gz" \
    | tar -xz --strip-components=1 -C /opt/openvscode-server \
 && ln -sf /opt/openvscode-server/bin/openvscode-server /usr/local/bin/openvscode-server

RUN python3 - <<'PY'
import json
from pathlib import Path
p = Path("/opt/openvscode-server/product.json")
data = json.loads(p.read_text(encoding="utf-8"))
data["extensionsGallery"] = {
    "serviceUrl": "https://marketplace.visualstudio.com/_apis/public/gallery",
    "itemUrl": "https://marketplace.visualstudio.com/items",
    "cacheUrl": "https://vscode.blob.core.windows.net/gallery/index",
    "controlUrl": ""
}
p.write_text(json.dumps(data, ensure_ascii=True, indent=2) + "\n", encoding="utf-8")
PY

RUN mkdir -p "${OPENVSCODE_DATA_DIR}/data/Machine" \
 && printf '%s\n' \
    '{' \
    '  "workbench.colorTheme": "Default Dark Modern",' \
    '  "python.defaultInterpreterPath": "/usr/local/bin/python3"' \
    '}' > "${OPENVSCODE_DATA_DIR}/data/Machine/settings.json" \
 && set -eux; \
    for ext in ms-python.python ms-python.vscode-pylance ms-python.debugpy ms-toolsai.jupyter charliermarsh.ruff; do \
      openvscode-server --install-extension "$ext" || true; \
    done

RUN useradd -m -s /bin/bash noryx \
 && usermod -aG sudo noryx \
 && echo 'noryx ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/90-noryx \
 && chmod 0440 /etc/sudoers.d/90-noryx \
 && mkdir -p /mnt /repos /datasets /home/noryx/.noryx-profile \
 && chown -R noryx:noryx /home/noryx /mnt /repos /datasets

EXPOSE 8888
ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["openvscode-server", "--host", "0.0.0.0", "--port", "8888", "--without-connection-token", "--telemetry-level", "off", "--default-folder", "/mnt"]
USER noryx
