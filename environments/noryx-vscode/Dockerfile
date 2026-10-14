FROM python:3.12-slim-bookworm

ENV DEBIAN_FRONTEND=noninteractive \
    OPENVSCODE_VERSION=1.109.5 \
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

# Idem, et celui-ci etait pire parce qu'il etait muet.
#
# Un RUN heredoc passait a sh sa commande sans
# son corps : python lisait une entree vide, ne faisait rien, et Kaniko
# notait "No files were changed". Toutes les images reconstruites dans le
# cluster partaient donc sans marketplace configure, et une extension
# installee par un utilisateur n'avait aucune galerie a interroger.
COPY environments/noryx-vscode/noryx-configure-extension-gallery /tmp/noryx-configure-extension-gallery
RUN python3 /tmp/noryx-configure-extension-gallery && rm -f /tmp/noryx-configure-extension-gallery

RUN useradd -m -s /bin/bash noryx \
 && usermod -aG sudo noryx \
 && echo 'noryx ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/90-noryx \
 && chmod 0440 /etc/sudoers.d/90-noryx \
 && mkdir -p /mnt /repos /datasets /extracts /home/noryx/.noryx-profile /opt/noryx-vscode/data/Machine /opt/noryx-vscode/extensions \
 && chown -R noryx:noryx /home/noryx /mnt /repos /datasets /extracts /opt/noryx-vscode

# Does every extension's runtime floor fit the Node this editor ships?
#
# The question that was never asked, and the one that cost the code assistant.
# An extension is installed on its "engines.vscode" alone; "engines.node" is
# checked by nobody. Continue 2.x wants Node 20.20.1, the editor pinned here
# shipped 20.18.3, so it installed, reported success, and never activated. The
# platform went on writing its configuration faithfully into a workspace where
# the assistant did not exist, and the loss looked like a platform fault.
#
# Nothing here pins the extensions, deliberately: they come from a registry
# and move on their own. What is pinned is the editor, so the mismatch is a
# build-time fact and this is where it belongs - a failed build leaves
# yesterday's image serving, which is a platform with an assistant.
# Copie d'un fichier du contexte, et non un heredoc.
#
# Ce bloc etait un COPY heredoc, de la syntaxe BuildKit. Docker
# l'accepte, donc l'image construite sur la VM de build etait juste et a
# ete deployee - et Kaniko, qui reconstruit les environnements dans le
# cluster chaque nuit, ne la connait pas : il echoue sur
# "failed to get files used from context: resolved envs is empty". Le
# cronjob noryx-environment-rebuild a donc echoue chaque nuit depuis le
# 2026-10-12, sans que rien ne le dise, l'alerte n'ayant pas de
# destination sur cette installation.
# Le chemin part de la racine du depot, parce que c'est le contexte des deux
# constructeurs : le cronjob passe --context=git://...NoryxLab-CE.git et le
# build documente est "docker build -f environments/noryx-vscode/Dockerfile .".
# Un nom de fichier nu aurait ete cherche a la racine, et manque.
COPY environments/noryx-vscode/noryx-check-extension-runtimes /usr/local/bin/noryx-check-extension-runtimes

USER noryx

# The editor's extensions, and a build that fails when one of them does not
# arrive.
#
# This loop used to end in "|| true". A registry that answered badly - or an
# extension whose identifier moved - therefore produced an image that built,
# published and started, missing a tool the product promises. That is what
# happened to the code assistant: Continue disappeared from the image, the
# nightly rebuild changed IMAGE_BUILD, and the bootstrap dutifully copied the
# new extension set over every profile that had one. The loss propagated to
# people who already had it working.
#
# Failing the build is the right answer precisely because the rebuild is
# nightly: a build that fails leaves yesterday's image in place, which is a
# working platform. A build that succeeds half-equipped replaces it.
#
# Mais chaque extension est demandee jusqu'a trois fois avant d'abandonner.
# Sans cela la stricture se retournait contre elle-meme : le 2026-10-14 la
# reconstruction est tombee sur "read ECONNRESET" en recuperant
# charliermarsh.ruff, et une coupure reseau d'une seconde a coute la nuit
# entiere. Refuser une image amputee est juste; refuser une image parce que
# le marketplace a hoquete ne l'est pas. Trois echecs de suite restent un
# echec, et le build s'arrete alors en nommant l'extension.
#
# The install is checked a second time against the disk, because reporting
# success and producing nothing are not the same thing and only one of them
# is visible here. Pylance is excluded from that check: it is proprietary and
# resolves to a different directory name depending on what the registry
# serves, so asserting its path would fail for a reason that is not a fault.
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
      installe=0; \
      for essai in 1 2 3; do \
        if openvscode-server --user-data-dir /opt/noryx-vscode/data --extensions-dir /opt/noryx-vscode/extensions --install-extension "$ext"; then installe=1; break; fi; \
        echo "extension $ext: tentative $essai echouee, nouvelle tentative dans 5s" >&2; \
        sleep 5; \
      done; \
      if [ "$installe" = 0 ]; then echo "extension $ext: abandon apres 3 tentatives" >&2; exit 1; fi; \
    done; \
    for ext in ms-python.python ms-python.debugpy ms-toolsai.jupyter charliermarsh.ruff Continue.continue; do \
      ls -d "/opt/noryx-vscode/extensions/$(echo "$ext" | tr "[:upper:]" "[:lower:]")"-* >/dev/null; \
    done; \
    /opt/openvscode-server/node --version > /opt/noryx-vscode/NODE_VERSION; \
    python3 /usr/local/bin/noryx-check-extension-runtimes

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
