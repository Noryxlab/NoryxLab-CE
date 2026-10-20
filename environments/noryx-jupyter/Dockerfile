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
    NORYX_WORKSPACE_IDE=jupyter \
    PIP_DISABLE_PIP_VERSION_CHECK=1 \
    XDG_CACHE_HOME=/home/noryx/.noryx-profile/.cache \
    PIP_CACHE_DIR=/home/noryx/.noryx-profile/.cache/pip \
    UV_CACHE_DIR=/home/noryx/.noryx-profile/.cache/uv \
    HF_HOME=/home/noryx/.noryx-profile/.cache/huggingface \
    RENV_PATHS_CACHE=/home/noryx/.noryx-profile/.cache/renv \
    PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1

RUN apt-get update \
 && apt-get install -y --no-install-recommends ca-certificates curl git sudo tini \
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
    httpx==0.27.2 \
    black==24.10.0 \
    isort==5.13.2 \
    ruff==0.8.1

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
