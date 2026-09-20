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

# conda, from conda-forge rather than Anaconda's own channels.
#
# Miniconda ships with Anaconda's own channels configured, and their terms
# require a paid licence for organisations past a couple of hundred people -
# which is every customer this platform has. A research platform that quietly
# puts its users in breach of a licence by providing the obvious tool is not a
# service to them, so the obvious tool here points at conda-forge and nothing
# else. Miniforge gives the same conda command people already know, plus mamba.
#
# The checksum is verified because this runs an installer downloaded over the
# network, as root, into the image every workspace starts from.
#
# Two details about making it reachable, both easy to get wrong. Only
# /opt/conda/condabin goes on PATH, never /opt/conda/bin: the latter carries
# conda's own python and would shadow the interpreter this image is built
# around, changing what python means for everybody who never asked for conda.
# And the shell hook is installed twice, which is not belt and braces. A
# terminal opened in the editor is an interactive non-login shell and reads
# /etc/bash.bashrc only; a login shell reads /etc/profile.d only, and resets
# PATH on the way, dropping the ENV above. Either one alone leaves conda
# reported as "command not found" in half the places somebody types it, and
# both halves were observed rather than reasoned about.
# The installer keeps its published name: the .sha256 beside it lists that
# filename, so renaming the download makes the check fail on a file it cannot
# find rather than on a mismatch.
ARG MINIFORGE_VERSION=26.7.2-0
ENV PATH=/opt/conda/condabin:$PATH
RUN base="https://github.com/conda-forge/miniforge/releases/download/${MINIFORGE_VERSION}" \
 && file="Miniforge3-${MINIFORGE_VERSION}-Linux-x86_64.sh" \
 && cd /tmp \
 && curl -fsSL "$base/$file" -o "$file" \
 && curl -fsSL "$base/$file.sha256" -o "$file.sha256" \
 && sha256sum -c "$file.sha256" \
 && bash "$file" -b -p /opt/conda \
 && rm -f "$file" "$file.sha256" \
 && /opt/conda/bin/conda config --system --set auto_activate_base false \
 && /opt/conda/bin/conda clean -afy \
 && printf '%s\n' 'export PATH=/opt/conda/condabin:$PATH' '. /opt/conda/etc/profile.d/conda.sh' > /etc/profile.d/noryx-conda.sh \
 && chmod 0644 /etc/profile.d/noryx-conda.sh \
 && printf '%s\n' '. /etc/profile.d/noryx-conda.sh' >> /etc/bash.bashrc

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

RUN useradd -m -s /bin/bash noryx \
 && usermod -aG sudo noryx \
 && echo 'noryx ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/90-noryx \
 && chmod 0440 /etc/sudoers.d/90-noryx \
 && mkdir -p /mnt /repos /datasets /home/noryx/.noryx-profile /opt/noryx-vscode/data/Machine /opt/noryx-vscode/extensions \
 && chown -R noryx:noryx /home/noryx /mnt /repos /datasets /opt/noryx-vscode

USER noryx

RUN mkdir -p "/opt/noryx-vscode/data/Machine" \
 && printf '%s\n' \
    '{' \
    '  "workbench.colorTheme": "Default Dark Modern",' \
    '  "python.defaultInterpreterPath": "/usr/local/bin/python3",' \
    '  "extensions.autoUpdate": false,' \
    '  "extensions.autoCheckUpdates": false,' \
    '  "continue.pauseCodebaseIndexOnStart": true' \
    '}' > "/opt/noryx-vscode/data/Machine/settings.json" \
 && set -eux; \
    for ext in ms-python.python ms-python.vscode-pylance ms-python.debugpy ms-toolsai.jupyter charliermarsh.ruff Continue.continue; do \
      openvscode-server --user-data-dir /opt/noryx-vscode/data --extensions-dir /opt/noryx-vscode/extensions --install-extension "$ext" || true; \
    done

# What the workspace bootstrap compares against, to know this image has moved.
#
# Extensions are copied into a person's profile volume on first launch and
# were then never refreshed, so a rebuilt image reached nobody who had already
# opened a workspace. The bootstrap now redelivers them when this value
# differs from the one the profile was last given.
#
# The nightly rebuild passes NORYX_BUILD_ID. Without it the build date is used,
# which freezes if this layer is cached - and a cached layer means nothing
# changed, so not redelivering is the right answer.
ARG NORYX_BUILD_ID=
RUN printf '%s\n' "${NORYX_BUILD_ID:-$(date -u +%Y%m%dT%H%M%SZ)}" > /opt/noryx-vscode/IMAGE_BUILD

EXPOSE 8888
ENTRYPOINT ["/usr/bin/tini", "--"]
CMD ["openvscode-server", "--host", "0.0.0.0", "--port", "8888", "--without-connection-token", "--telemetry-level", "off", "--default-folder", "/mnt"]
