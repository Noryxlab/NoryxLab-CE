FROM python:3.12-slim-bookworm

# Les caches vont sur le volume personnel, pas dans l'ephemere.
#
# Trois chemins seulement survivent a l'arret d'un workspace : /mnt (le
# projet), /home/noryx/.noryx-profile (la personne) et /datasets. Tout le
# reste, dossier personnel compris, est efface - et decompte du plafond de
# stockage ephemere du pod.
#
# pip, uv et huggingface ecrivent par defaut sous ~/.cache, donc dans
# l'ephemere : un environnement Python un peu fourni remplissait le quota sans
# que rien ne le dise, la session etait evincee, et ce qui avait ete installe
# aurait de toute facon disparu a l'arret suivant. Rencontre sur EMSE le
# 2026-10-20 : deux symptomes pour une seule cause.
#
# Les pointer sur le volume personnel regle les deux : le quota ephemere cesse
# de se remplir, et un paquet telecharge une fois est retrouve a la session
# suivante.
ENV DEBIAN_FRONTEND=noninteractive \
    OPENVSCODE_VERSION=1.99.3 \
    OPENVSCODE_DATA_DIR=/home/noryx/.openvscode-server \
    NORYX_AUTO_UPDATE_IDE=0 \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    XDG_CACHE_HOME=/home/noryx/.noryx-profile/.cache \
    PIP_CACHE_DIR=/home/noryx/.noryx-profile/.cache/pip \
    UV_CACHE_DIR=/home/noryx/.noryx-profile/.cache/uv \
    HF_HOME=/home/noryx/.noryx-profile/.cache/huggingface \
    RENV_PATHS_CACHE=/home/noryx/.noryx-profile/.cache/renv \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

# Where a user's own packages are installed.
#
# pip finds the interpreter's site-packages read-only - it belongs to the image -
# and says so before falling back to a user install: "Defaulting to user
# installation because normal site-packages is not writeable". That fallback
# landed in ~/.local, which is the pod's ephemeral disk: a few large wheels
# (torch and its CUDA dependencies) filled the 8Gi allowance and the session was
# evicted mid-install, having also lost every package at the previous restart.
# Pointing the user base at the profile makes an install persist and stop
# counting against the ephemeral allowance; a virtual environment is unaffected,
# which is why PIP_USER is deliberately not set (pip refuses a user install
# inside a venv).
ENV PYTHONUSERBASE=/home/noryx/.noryx-profile/.local \
    NPM_CONFIG_PREFIX=/home/noryx/.noryx-profile/.local \
    PATH=/home/noryx/.noryx-profile/.local/bin:$PATH

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

# Use Microsoft VS Code Marketplace instead of OpenVSX for extension install/update.
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
 && mkdir -p /mnt /repos /datasets /extracts /home/noryx/.noryx-profile \
 && chown -R noryx:noryx /home/noryx /mnt /repos /datasets /extracts

EXPOSE 8888

ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["python3", "-m", "jupyterlab", "--ip=0.0.0.0", "--port=8888", "--no-browser"]

USER noryx
